package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/supervisor"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop processes",
	Run:   execStopCmd,
}

func init() {
	stopCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execStopPersistentPreRun()
	}

	rootCmd.AddCommand(stopCmd)
}

func execStopPersistentPreRun() {
	requireDaemonRunning()
}

func execStopCmd(cmd *cobra.Command, args []string) {
	msg.Action = supervisor.ActionStop
	msg.Processes = parseProcessArgs(args, ";")

	res := supervisor.ClientRun(msg)
	if res == nil {
		fmt.Println("No processes to stop.")
		return
	}

	for _, proc := range res {
		fmt.Printf("[%s] Stopped %s\t[PID %d]\n", time.UnixMilli(proc.StopAt).Format(time.RFC3339), proc.Name, proc.Pid)
	}
}
