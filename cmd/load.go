package cmd

import (
	"spm/pkg/client"
	"spm/pkg/config"

	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:     "load",
	Aliases: []string{"update"},
	Short:   "load all project snapshots",
	Run:     execLoadCmd,
}

func init() {
	setupCommandPreRun(loadCmd, requireDaemonRunning)
	rootCmd.AddCommand(loadCmd)
}

func execLoadCmd(cmd *cobra.Command, args []string) {
	_ = client.Load(config.WorkDirFlag, config.ProcfileFlag)
}
