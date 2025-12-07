package cmd

import (
	"fmt"
	"spm/pkg/supervisor"
	"time"

	"github.com/spf13/cobra"
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
	msg.Action = supervisor.ActionShutdown

	// 使用 channel 异步执行 RPC 调用
	done := make(chan struct{})
	go func() {
		_ = supervisor.ClientRun(msg)
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
