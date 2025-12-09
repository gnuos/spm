package cmd

import (
	"fmt"
	"time"

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

	var uptime string
	nopTime := time.Time{}

	for _, proc := range res {
		now := time.Now()
		aliveTime := proc.StopAt.Sub(proc.StartAt)
		if proc.StopAt.Equal(nopTime) && !proc.StartAt.Equal(nopTime) {
			aliveTime = now.Sub(proc.StartAt)
		}

		if aliveTime < 0 {
			aliveTime = 0 * time.Second
		}

		if aliveTime.Hours() >= 1 {
			uptime = fmt.Sprintf("%dh:%dm:%ds.%dms", int64(aliveTime.Hours()), int64(aliveTime.Minutes())%60, int64(aliveTime.Seconds())%60, int(aliveTime.Milliseconds())%1000)
		} else if aliveTime.Minutes() >= 1 {
			uptime = fmt.Sprintf("%dm:%ds.%dms", int(aliveTime.Minutes()), int64(aliveTime.Seconds())%60, int(aliveTime.Milliseconds())%1000)
		} else if aliveTime.Seconds() > 0 {
			uptime = fmt.Sprintf("%ds.%dms", int64(aliveTime.Seconds())%60, int(aliveTime.Milliseconds())%1000)
		} else {
			uptime = "0s"
		}

		fmt.Printf("Project: %s\tProcess: %s\t\tState: %s\t\tPID: %d\t\tUptime: %s\n", proc.Project, proc.Name, proc.Status, proc.Pid, uptime)
	}
}
