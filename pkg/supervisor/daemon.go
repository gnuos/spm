// Package supervisor 提供 Daemon 管理功能
package supervisor

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"spm/pkg/config"
	"spm/pkg/utils"

	"github.com/gnuos/daemon"
)

var daemonCtx *daemon.Context

// GetDaemon 获取或创建 Daemon 上下文
//
// 返回：
//
//	*daemon.Context: Daemon 上下文实例
//
// 注意：
//
//	使用单例模式，首次调用时创建，后续调用返回同一实例
//
// 创建时间: 2025-12-06
func GetDaemon() *daemon.Context {
	if daemonCtx == nil {
		daemonCtx = &daemon.Context{
			PidFileName: config.GetConfig().PidFile,
			PidFilePerm: 0644,
			WorkDir:     config.WorkDirFlag,
			Umask:       027,
			Args:        os.Args,
		}
	}

	return daemonCtx
}

// Daemon 以守护进程模式运行 Supervisor
//
// 功能：
//  1. 初始化守护进程（或前台模式）
//  2. 启动 RPC 服务器
//  3. 监听系统信号
//  4. 优雅关闭
//
// 运行模式：
//   - 前台模式（config.ForegroundFlag = true）：直接运行
//   - 后台模式（config.ForegroundFlag = false）：使用 daemon.Reborn() 创建守护进程
//
// 信号处理：
//   - SIGINT/SIGTERM: 触发优雅关闭
//   - SIGQUIT: 触发优雅关闭
//
// 清理逻辑：
//
//	使用 defer 确保退出时清理 PID 文件和 Socket 文件
//
// 注意事项：
//  1. 此方法会阻塞，直到收到终止信号
//  2. 父进程会在子进程创建后立即返回
//
// 示例：
//
//	sv := NewSupervisor()
//	sv.Daemon()  // 阻塞运行
func (sv *Supervisor) Daemon() {
	defer func() {
		if config.ForegroundFlag {
			_ = os.Remove(config.GetConfig().PidFile)
		} else {
			_ = GetDaemon().Release()
		}
		_ = os.Remove(config.GetConfig().Socket)
	}()

	sv.StartedAt = time.Now()

	if config.ForegroundFlag {
		err := utils.WriteDaemonPid(utils.SupervisorPid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	} else {
		d, err := GetDaemon().Reborn()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			_ = GetDaemon().Release()
			os.Exit(1)
		}

		if d != nil {
			sv.Pid = d.Pid
			return
		}
	}

	fmt.Printf("\033[1;33;40mSpm supervisor started at %s\033[0m\n\n", sv.StartedAt.Format(time.RFC3339))

	go StartServer(sv)

	sv.logger.Infof("Spm supervisor PID %d", sv.Pid)

	if config.ForegroundFlag {
		go sv.AfterStart()
	}

	sig := <-utils.StopChan

	switch sig {
	case os.Interrupt, syscall.SIGTERM:
		utils.FinishChan <- struct{}{}
		sv.Shutdown()
	}
	close(utils.StopChan)

	sv.logger.Info("Supervisor daemon stopped")
}

// Shutdown 优雅关闭 Supervisor
//
// 功能：
//  1. 停止所有进程
//  2. 同步所有进程日志
//  3. 记录关闭日志
//
// 注意事项：
//
//	此方法通常由 Daemon() 在收到信号时调用，不应直接调用
//
// 示例：
//
//	defer sv.Shutdown()  // 确保程序退出时调用
func (sv *Supervisor) Shutdown() {
	_ = sv.StopAll("*")

	pt := sv.procTable.Iter()
	for _, p := range pt {
		_ = p.logger.Sync()
	}

	sv.logger.Info("Shutdown supervisor...")
}
