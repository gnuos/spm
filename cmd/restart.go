package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/supervisor"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart processes",
	Run:   execRestartCmd,
}

func init() {
	stopCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execRestartPersistentPreRun()
	}

	rootCmd.AddCommand(restartCmd)
}

func execRestartPersistentPreRun() {
	requireDaemonRunning()
}

func execRestartCmd(cmd *cobra.Command, args []string) {
	msg.Action = supervisor.ActionRestart
	msg.Processes = parseProcessArgs(args, ";")

	res := supervisor.ClientRun(msg)
	if res == nil {
		fmt.Println("No processes to restart.")
		return
	}

	for _, proc := range res {
		fmt.Printf("[%s] Restarted %s\t[PID %d]\n", time.UnixMilli(proc.StartAt).Format(time.RFC3339), proc.Name, proc.Pid)
	}
}
