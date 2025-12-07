package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/client"
	"spm/pkg/config"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop processes",
	Run:   execStopCmd,
}

func init() {
	setupCommandPreRun(stopCmd, requireDaemonRunning)
	rootCmd.AddCommand(stopCmd)
}

func execStopCmd(cmd *cobra.Command, args []string) {
	res := client.Stop(config.WorkDirFlag, config.ProcfileFlag, args...)
	if res == nil {
		fmt.Println("No processes to stop.")
		return
	}

	for _, proc := range res {
		fmt.Printf("[%s] %s\t[PID %d] %s\n", time.UnixMilli(proc.StopAt).Format(time.RFC3339), proc.Name, proc.Pid, proc.Status)
	}
}
