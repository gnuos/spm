package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"spm/pkg/supervisor"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check processed status",
	Run:   execStatusCmd,
}

func init() {
	statusCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execStatusPersistentPreRun()
	}

	rootCmd.AddCommand(statusCmd)
}

func execStatusPersistentPreRun() {
	requireDaemonRunning()
}

func execStatusCmd(cmd *cobra.Command, args []string) {
	msg.Action = supervisor.ActionStatus
	msg.Processes = parseProcessArgs(args, ";")

	res := supervisor.ClientRun(msg)
	if res == nil {
		fmt.Println("No processes found.")
		return
	}

	for _, proc := range res {
		fmt.Printf("%s\t\t%s\t\tPID: %d\n", proc.Name, proc.Status, proc.Pid)
	}
}
