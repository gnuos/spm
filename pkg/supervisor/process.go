// Package supervisor
package supervisor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"spm/pkg/codec"
	"spm/pkg/config"
	"spm/pkg/logger"
	"spm/pkg/utils"

	"go.uber.org/zap"
)

var sigTable = map[string]syscall.Signal{
	"INT":   syscall.SIGINT,
	"TERM":  syscall.SIGTERM,
	"QUIT":  syscall.SIGQUIT,
	"STOP":  syscall.SIGSTOP,
	"ABORT": syscall.SIGABRT,
}

type Process struct {
	Pid      int
	Name     string
	FullName string
	PidPath  string
	OutLog   string
	ErrLog   string
	StartAt  time.Time
	StopAt   time.Time
	State    codec.ProcessState

	// 进程的配置参数，不对外暴露
	opts *ProcessOption

	// 进程和goroutine上下文
	mu     sync.Mutex
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	logger  *zap.SugaredLogger
	signal  syscall.Signal
	sysproc *os.Process
	stdout  io.ReadWriteCloser
	stderr  io.ReadWriteCloser
}

func NewProcess(fullName string, opts *ProcessOption) *Process {
	stopSignal, ok := sigTable[opts.StopSignal]
	if !ok {
		// 默认用SIGINT信号关闭子进程，可以平滑退出
		stopSignal = sigTable["INT"]
	}

	name := strings.Split(fullName, "::")[1]
	logDir := config.GetRuntimeDir(opts.Root)

	if opts.LogRoot != "" {
		logDir = opts.LogRoot
	}

	outputLogPath := fmt.Sprintf("%s/%s_output.log", logDir, name)
	errorLogPath := fmt.Sprintf("%s/%s_error.log", logDir, name)

	return &Process{
		Pid:      0,
		Name:     name,
		FullName: fullName,
		OutLog:   outputLogPath,
		ErrLog:   errorLogPath,

		StartAt: time.Time{},
		StopAt:  time.Time{},
		State:   codec.ProcessStandby,

		opts:   opts,
		signal: stopSignal,
		logger: logger.Logging(fullName),
	}
}

func (p *Process) SetPidPath() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.PidPath = fmt.Sprintf("%s/%s.pid", config.GetRuntimeDir(p.opts.Root), p.Name)

	if p.opts.PidRoot != "" {
		info, err := os.Stat(p.opts.PidRoot)
		if err == nil && info.IsDir() {
			p.PidPath = fmt.Sprintf("%s/%s.pid", p.opts.PidRoot, p.Name)
		}
	}
}

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sysproc == nil {
		p.State = codec.ProcessStandby
		return false
	}

	if p.Pid > 0 {
		process, err := os.FindProcess(p.Pid)
		if err != nil {
			p.State = codec.ProcessStopped
			return false
		}

		// 发送信号0来检查进程是否存活
		if err = process.Signal(syscall.Signal(0)); err != nil {
			p.State = codec.ProcessStopped
			return false
		}
	}

	p.State = codec.ProcessRunning
	return true
}

func (p *Process) Status() codec.ProcessState {
	return p.State
}

// validateStart 验证进程是否可以启动
func (p *Process) validateStart() error {
	if p.State == codec.ProcessStopping {
		return fmt.Errorf("process is stopped/stopping therefore cannot be started again")
	}

	if p.IsRunning() {
		p.logger.Warnf("Already running with PID %d", p.Pid)
		return fmt.Errorf("process already running with PID %d", p.Pid)
	}

	return nil
}

// prepareEnvironment 准备启动环境（日志文件和工作目录）
func (p *Process) prepareEnvironment() error {
	// 切换到进程的工作目录
	if err := os.Chdir(p.opts.Root); err != nil {
		return fmt.Errorf("cannot change to working directory %s: %w", p.opts.Root, err)
	}

	outLog, err := os.OpenFile(p.OutLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		p.logger.Error(err)
		return fmt.Errorf("cannot open log files: %v", err)
	}

	// 每次启动都打开日志文件描述符
	errLog, err := os.OpenFile(p.ErrLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		_ = outLog.Close() // 第一个文件已打开，需要关闭防止资源泄漏
		p.logger.Error(err)
		return fmt.Errorf("cannot open log files: %v", err)
	}

	p.stdout = outLog
	p.stderr = errLog

	return nil
}

// buildCommand 构建要执行的命令
func (p *Process) buildCommand() (*exec.Cmd, error) {
	task := p.opts.Cmd
	if len(task) == 0 {
		return nil, fmt.Errorf("command is empty")
	}

	// 解析命令和参数
	exe := task[0]
	var args []string
	if len(task) > 1 {
		args = task[1:]
	}

	// 创建带取消功能的上下文
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel

	// 构建命令
	cmd := exec.CommandContext(p.ctx, exe, args...)
	cmd.Env = append(cmd.Env, p.opts.Env...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return cmd, nil
}

// setupStreams 设置标准输出和错误输出的管道，并启动日志监控
func (p *Process) setupStreams(cmd *exec.Cmd) error {
	// 创建标准输出管道
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// 创建标准错误管道
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 启动日志监控 goroutine
	p.wg.Add(2)
	go p.watchLog("STDOUT", stdoutPipe)
	go p.watchLog("STDERR", stderrPipe)

	return nil
}

// launchProcess 启动进程并记录状态
func (p *Process) launchProcess(cmd *exec.Cmd) error {
	// 启动进程
	if err := cmd.Start(); err != nil {
		p.State = codec.ProcessFailed
		return fmt.Errorf("failed to start process: %w", err)
	}

	// 记录进程信息
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Pid = cmd.Process.Pid
	p.sysproc = cmd.Process
	p.StartAt = time.Now()
	p.StopAt = time.Time{}
	p.State = codec.ProcessRunning

	// 写入PID文件
	if err := os.WriteFile(p.PidPath, []byte(strconv.Itoa(p.Pid)), 0644); err != nil {
		p.logger.Error(err)
	}

	return nil
}

// monitorProcess 在goroutine中监控进程，等待其结束并处理退出状态
func (p *Process) monitorProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			p.logger.Error(err)
		} else {
			ws := exitErr.Sys().(syscall.WaitStatus)
			if ws.Signaled() {
				p.logger.Infof("%v process %s ", p.signal, p.Name)
			} else {
				p.logger.Infof("process %s exited with code=%d", p.Name, ws.ExitStatus())
			}
		}
	}

	p.mu.Lock()
	p.onStop()
	p.StopAt = time.Now()
	p.State = codec.ProcessStopped
	p.mu.Unlock()
}

func (p *Process) Start() bool {
	// 验证启动条件
	if err := p.validateStart(); err != nil {
		// 如果已经在运行，返回 true（这是预期行为）
		if p.IsRunning() {
			return true
		}
		p.logger.Warn(err)
		return false
	}

	// 准备环境（日志文件和工作目录）
	if err := p.prepareEnvironment(); err != nil {
		p.logger.Error(err)
		return false
	}

	// 构建命令
	cmd, err := p.buildCommand()
	if err != nil {
		p.logger.Error(err)
		return false
	}

	// 设置输出流管道
	if err := p.setupStreams(cmd); err != nil {
		p.logger.Error(err)
		return false
	}

	// 启动进程
	if err := p.launchProcess(cmd); err != nil {
		p.logger.Error(err)
		return false
	}

	// 在后台监控进程
	go p.monitorProcess(cmd)

	p.logger.Infof("Process %s is started", p.Name)
	return true
}

func (p *Process) Stop() bool {
	if p.IsRunning() && !p.updatePid() {
		p.State = codec.ProcessUnknown
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	switch p.State {
	case codec.ProcessRunning:
		{
			if p.cancel != nil {
				p.cancel()

				p.wg.Wait()
			}

			p.State = codec.ProcessStopping

			timer := time.NewTimer(3 * time.Second)
			select {
			case <-p.ctx.Done():
				p.logger.Infof("Process %s exited gracefully", p.Name)
			case <-timer.C:
				p.logger.Warnf("Process %s exited timeout", p.Name)
			}

			err := p.ctx.Err()
			if err != nil && !errors.Is(err, context.Canceled) {
				p.logger.Error(err)
			} else {
				p.logger.Infof("Sending %s to PID %d", p.opts.StopSignal, p.Pid)
				err = syscall.Kill(-p.Pid, p.signal)
				if err != nil && !errors.Is(err, os.ErrProcessDone) {
					p.logger.Error(err)
				} else {
					err = nil
				}
			}

			if err != nil {
				p.logger.Warnf("Force kill process %s", p.Name)
				_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
			}

			p.State = codec.ProcessStopped
			p.onStop()
		}
	case codec.ProcessStopped:
		p.logger.Infof("Process %s already stopped", p.Name)
	default:
		p.logger.Infof("Process %s status is %s", p.Name, p.State)
	}

	return p.State == codec.ProcessStopped
}

func (p *Process) Restart() bool {
	_ = p.updatePid()
	if p.IsRunning() {
		_ = p.Stop()
	} else {
		p.logger.Infof("Process %s is not running. Starting it.", p.Name)
		p.mu.Lock()
		p.onStop()
		p.mu.Unlock()
	}

	p.mu.Lock()
	p.Pid = 0
	p.mu.Unlock()

	return p.Start()
}

func (p *Process) updatePid() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	pid, err := utils.ReadPid(p.PidPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			p.logger.Error(err)
		}
		return false
	}

	if pid > 0 && pid != p.Pid {
		p.Pid = pid
		fInfo, err := os.Stat(p.PidPath)
		if err != nil {
			p.logger.Error(err)
			return false
		}

		p.StartAt = fInfo.ModTime()

		p.logger.Debugf("PID file changed. Updated pid to %d", p.Pid)
	}

	return true
}

func (p *Process) onStop() {
	p.StartAt = time.Time{}

	err := os.Remove(p.PidPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			p.logger.Error(err)
		}
	} else {
		p.logger.Debugf("Removed %s PID file", p.Name)
	}
}

func (p *Process) watchLog(logtype string, r io.ReadCloser) {
	defer p.wg.Done()

	tty := os.Stdout
	dest := p.stdout
	if logtype == "STDERR" {
		dest = p.stderr
		tty = os.Stderr
	}

	defer func() {
		if err := dest.Close(); err != nil {
			p.logger.Warnf("%s log file close error: %v", logtype, err)
		}
		p.logger.Infof("%s logging finished", logtype)
	}()

	tee := io.TeeReader(r, dest)

	scanner := bufio.NewScanner(tee)
	for scanner.Scan() {
		line := scanner.Text()
		if config.ForegroundFlag {
			_, _ = fmt.Fprintf(tty, "%s\n", line)
		}
	}

	err := scanner.Err()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		p.logger.Error(err)
	}
}
