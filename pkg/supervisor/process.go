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

	"go.uber.org/zap"
)

type ProcessState string

const (
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

func (p *Process) Start() bool {
	if p.State == processStopping {
		p.logger.Warn("Process is stopped/stopping therefore cannot be started again.")
		return false
	}

	if p.IsRunning() {
		p.logger.Warnf("Already running with PID %d", p.Pid)
		return true
	}

	// 每次启动都赋值打开日志文件描述符
	if !p.SetLog() {
		p.logger.Error("Cannot open log files")

		return false
	}

	err := os.Chdir(p.Options.Root)
	if err != nil {
		p.logger.Error(err)
		return false
	}

	var args []string
	task := p.Options.cmd
	exe := task[0]
	if len(task) > 1 {
		args = task[1:]
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel

	cmd := exec.CommandContext(p.ctx, exe, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.Error(err)
		return false
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		p.logger.Error(err)
		return false
	}

	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(cmd.Env, p.Env...)

	p.wg.Add(2)
	go p.watchLog("STDOUT", stdoutPipe)
	go p.watchLog("STDERR", stderrPipe)

	if err := cmd.Start(); err != nil {
		p.State = processFailed
		p.logger.Error(err)
		return false
	}

	p.mu.Lock()
	p.Pid = cmd.Process.Pid
	p.sysproc = cmd.Process
	p.StartAt = time.Now()
	p.State = processRunning

	err = os.WriteFile(p.pidPath, []byte(strconv.Itoa(p.Pid)), 0644)
	if err != nil {
		p.logger.Error(err)
	}

	p.mu.Unlock()

	go func(c *exec.Cmd) {
		err := c.Wait()
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
	}(cmd)

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
