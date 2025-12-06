package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/supervisor"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload processes and options",
	Run:   execReloadCmd,
}

func init() {
	reloadCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execReloadPersistentPreRun()
	}

	rootCmd.AddCommand(reloadCmd)
}

func execReloadPersistentPreRun() {
	requireDaemonRunning()
}

func execReloadCmd(cmd *cobra.Command, args []string) {
	msg.Action = supervisor.ActionReload

	res := supervisor.ClientRun(msg)
	if res == nil {
		fmt.Println("No processes changed")
		return
	}

	for _, proc := range res {
		fmt.Printf("[%s] Load %s\t%s\n", time.Now().Format(time.RFC3339), proc.Name, proc.Status)
	}
}
