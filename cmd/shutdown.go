package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/client"
	"spm/pkg/config"
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Stop supervisor",
	Run:   execShutdownCmd,
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
		close(done)
	}()

	// 等待 RPC 响应或超时
	select {
	case <-done:
		fmt.Println("Supervisor service has been stopped.")
	case <-time.After(5 * time.Second):
		fmt.Println("Shutdown initiated (timeout waiting for response).")
	}
}
