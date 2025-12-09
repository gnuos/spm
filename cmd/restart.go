package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/client"
	"spm/pkg/config"
)

var restartCmd = &cobra.Command{
	Use:     "restart",
	Short:   "Restart processes",
	Aliases: []string{"rs"},
	Run:     execRestartCmd,
}

func init() {
	setupCommandPreRun(restartCmd, requireDaemonRunning)
	rootCmd.AddCommand(restartCmd)
}

func execRestartCmd(cmd *cobra.Command, args []string) {
	res := client.Restart(config.WorkDirFlag, config.ProcfileFlag, args...)
	if res == nil {
		fmt.Println("No processes to restart.")
		return
	}

	for _, proc := range res {
		fmt.Printf("[%s] Restarted %s\t[PID %d]\n", proc.StartAt.Format(time.RFC3339), proc.Name, proc.Pid)
	}
}
