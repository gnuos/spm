package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/client"
	"spm/pkg/config"
)

var shutdownCmd = &cobra.Command{
	Use:     "shutdown",
	Short:   "Stop supervisor",
	Aliases: []string{"kill", "exit", "s"},
	Run:     execShutdownCmd,
}

func init() {
	setupCommandPreRun(shutdownCmd, requireDaemonRunning)
	rootCmd.AddCommand(shutdownCmd)
}

func execShutdownCmd(cmd *cobra.Command, args []string) {
	// 使用 channel 异步执行 RPC 调用
	done := make(chan struct{})
	go func() {
		_ = client.Shutdown(config.WorkDirFlag, config.ProcfileFlag)
		done <- struct{}{}
		close(done)
	}()

	// 等待 RPC 响应或超时
	select {
	case <-done:
		fmt.Println("All processes has been stopped.")
		killDaemon(syscall.SIGQUIT)
	case <-time.After(5 * time.Second):
		fmt.Println("Shutdown processes timeout. Force kill supervisor.")
		killDaemon(syscall.SIGKILL)
		_ = os.Remove(config.GetConfig().PidFile)
		_ = os.Remove(config.GetConfig().Socket)
	}

	fmt.Println("Supervisor service has been stopped.")
}
