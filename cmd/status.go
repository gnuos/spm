package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"spm/pkg/client"
	"spm/pkg/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check processed status",
	Run:   execStatusCmd,
}

func init() {
	setupCommandPreRun(statusCmd, requireDaemonRunning)
	rootCmd.AddCommand(statusCmd)
}

func execStatusCmd(cmd *cobra.Command, args []string) {
	res := client.Status(config.WorkDirFlag, config.ProcfileFlag, args...)
	if res == nil {
		fmt.Println("No processes found.")
		return
	}

	for _, proc := range res {
		fmt.Printf("%s\t\t%s\t\tPID: %d\n", proc.Name, proc.Status, proc.Pid)
	}
}
