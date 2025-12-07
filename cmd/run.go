package cmd

import (
	"fmt"
	"log"
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
	runCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execRunPersistentPreRun()
	}

	rootCmd.AddCommand(runCmd)
}

func execRunPersistentPreRun() {
	if !isDaemonRunning() {
		log.Fatalln("ERROR: Supervisor has not started. Please check supervisor daemon.")
	}
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
		fmt.Printf("[%s] Run %s\t[PID %d]\n", time.UnixMilli(res[0].StartAt).Format(time.RFC3339), res[0].Name, res[0].Pid)
	}
}
