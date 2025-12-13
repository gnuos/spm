package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/client"
	"spm/pkg/config"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run command as a process",
	Run:   execRunCmd,

	SilenceUsage: true,
}

func init() {
	setupCommandPreRun(runCmd, requireDaemonRunning)
	rootCmd.AddCommand(runCmd)
}

func execRunCmd(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		_ = cmd.Usage()
		return
	}

	res := client.Run(config.WorkDirFlag, config.ProcfileFlag, args)
	if res == nil {
		fmt.Println("No processes to run.")
		return
	}

	if len(res) > 0 {
		fmt.Printf("%s %s::%s\t[PID %d] %s\n", res[0].StartAt.Format(time.RFC3339), res[0].Project, res[0].Name, res[0].Pid, res[0].Status)
	}
}
