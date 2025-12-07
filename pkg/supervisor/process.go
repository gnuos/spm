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

	"spm/pkg/config"
	"spm/pkg/logger"
	"spm/pkg/utils"

	_ "github.com/k0kubun/pp/v3"
	"go.uber.org/zap"
)

type ProcessState string

const (
	processStarted  ProcessState = "Started"
	processNotfound ProcessState = "NotFound"
	processUnknown  ProcessState = "Unknown"
	processStopped  ProcessState = "Stopped"
	processStopping ProcessState = "Stopping"
	processRunning  ProcessState = "Running"
	processStandby  ProcessState = "Standby"
	processFailed   ProcessState = "Failed"
)

var sigTable = map[string]syscall.Signal{
	"INT":   syscall.SIGINT,
	"TERM":  syscall.SIGTERM,
	"QUIT":  syscall.SIGQUIT,
	"STOP":  syscall.SIGSTOP,
	"ABORT": syscall.SIGABRT,
}

var notFoundProc = &Process{
	Pid:      -1,
	FullName: "",
	StartAt:  time.Time{},
	StopAt:   time.Time{},
	State:    processNotfound,
}

type Process struct {
	Pid      int
	Name     string
	FullName string
	Options  *ProcessOption
	StartAt  time.Time
	StopAt   time.Time
	State    ProcessState
	OutLog   io.WriteCloser
	ErrLog   io.WriteCloser
	Env      []string

	wg      sync.WaitGroup
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.SugaredLogger
	signal  syscall.Signal
	sysproc *os.Process
	pidPath string
}

func NewProcess(fullName string, opts *ProcessOption) *Process {
	stopSignal, ok := sigTable[opts.StopSignal]
	if !ok {
		stopSignal = sigTable["TERM"]
	}

	env := make([]string, 0)
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	name := strings.Split(fullName, "::")[1]

	return &Process{
		Pid:      -1,
		Name:     name,
		FullName: fullName,
		Options:  opts,
		StartAt:  time.Time{},
		StopAt:   time.Time{},
		State:    processStandby,
		Env:      env,

		signal: stopSignal,
		logger: logger.Logging(fullName),
	}
}

func (p *Process) SetPidPath() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pidPath = fmt.Sprintf("%s/%s.pid", config.GetRuntimeDir(p.Options.Root), p.Name)

	if p.Options.PidRoot != "" {
		info, err := os.Stat(p.Options.PidRoot)
		if err == nil && info.IsDir() {
			p.pidPath = fmt.Sprintf("%s/%s.pid", p.Options.PidRoot, p.Name)
		}
	}
}

func (p *Process) SetLog() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	logDir := config.GetRuntimeDir(p.Options.Root)
	if p.Options.LogRoot != "" {
		logDir = p.Options.LogRoot
	}

	outputLogPath := fmt.Sprintf("%s/%s_output.log", logDir, p.Name)
	errorLogPath := fmt.Sprintf("%s/%s_error.log", logDir, p.Name)

	outLog, err := os.OpenFile(outputLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		p.logger.Error(err)
		return false
	}

	errLog, err := os.OpenFile(errorLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		_ = outLog.Close() // 第一个文件已打开，需要关闭防止资源泄漏
		p.logger.Error(err)
		return false
	}

	p.OutLog = outLog
	p.ErrLog = errLog

	return true
}

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sysproc == nil {
		p.State = processStandby
		return false
	}

	if p.Pid > 0 {
		process, err := os.FindProcess(p.Pid)
		if err != nil {
			p.State = processStopped
			return false
		}

		// 发送信号0来检查进程是否存活
		if err = process.Signal(syscall.Signal(0)); err != nil {
			p.State = processStopped
			return false
		}
	}

	p.State = processRunning
	return true
}

func (p *Process) Status() ProcessState {
	return p.State
}

// validateStart 验证进程是否可以启动
func (p *Process) validateStart() error {
	if p.State == processStopping {
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
	// 每次启动都打开日志文件描述符
	if !p.SetLog() {
		return fmt.Errorf("cannot open log files")
	}

	// 切换到进程的工作目录
	if err := os.Chdir(p.Options.Root); err != nil {
		return fmt.Errorf("cannot change to working directory %s: %w", p.Options.Root, err)
	}

	return nil
}

// buildCommand 构建要执行的命令
func (p *Process) buildCommand() (*exec.Cmd, error) {
	task := p.Options.cmd
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
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(cmd.Env, p.Env...)

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
		p.State = processFailed
		return fmt.Errorf("failed to start process: %w", err)
	}

	// 记录进程信息
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Pid = cmd.Process.Pid
	p.sysproc = cmd.Process
	p.StartAt = time.Now()
	p.State = processRunning

	// 写入PID文件
	if err := os.WriteFile(p.pidPath, []byte(strconv.Itoa(p.Pid)), 0644); err != nil {
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
				p.logger.Infof("Process %s is stopped by signal: %v", p.Name, p.signal)
			} else {
				p.logger.Infof("Process %s exited with code=%d", p.Name, ws.ExitStatus())
			}
		}
	}

	p.StopAt = time.Now()
	p.onStop()
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
		p.State = processUnknown
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.onStop()

	switch p.State {
	case processRunning:
		{
			if p.cancel == nil {
				return false
			}

			p.cancel()
			p.wg.Wait()

			p.logger.Infof("Sending %s to %d", p.Options.StopSignal, p.Pid)
			p.State = processStopping

			err := p.sysproc.Signal(p.signal)
			if err != nil && !errors.Is(err, os.ErrProcessDone) {
				p.logger.Error(err)
				_ = p.sysproc.Kill()
				p.State = processStopped
			} else {
				p.State = processStopped
			}
		}
	case processStopped:
		p.logger.Infof("Process %s already stopped", p.Name)
	default:
		p.logger.Infof("Process %s status is %s", p.Name, p.State)
	}

	return p.State == processStopped
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
	p.Pid = -1
	p.mu.Unlock()

	return p.Start()
}

func (p *Process) updatePid() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	pid, err := utils.ReadPid(p.pidPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			p.logger.Error(err)
		}
		return false
	}

	if pid > 0 && pid != p.Pid {
		p.Pid = pid
		fInfo, err := os.Stat(p.pidPath)
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

	err := os.Remove(p.pidPath)
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
	dest := p.OutLog
	if logtype == "STDERR" {
		dest = p.ErrLog
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
