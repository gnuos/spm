package cmd

import (
	"spm/pkg/client"
	"spm/pkg/config"

	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:     "dump",
	Aliases: []string{"save"},
	Short:   "dump all processes",
	Long:    "Save all project and processes to file for resume process list",
	Run:     execDumpCmd,
}

func init() {
	setupCommandPreRun(dumpCmd, requireDaemonRunning)
	rootCmd.AddCommand(dumpCmd)
}

func execDumpCmd(cmd *cobra.Command, args []string) {
	_ = client.Dump(config.WorkDirFlag, config.ProcfileFlag)
}
