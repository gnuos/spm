package cmd

import (
	"fmt"
	"log"

	"spm/pkg/config"
	"spm/pkg/supervisor"
	"spm/pkg/utils"
	"spm/pkg/utils/constants"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run supervisor as a daemon",
	Run:   execDaemonCmd,
}

func init() {
	daemonCmd.PersistentFlags().BoolVarP(&config.ForegroundFlag, "foreground", "f", false, "Run the supervisor in the foreground")

	daemonCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execDaemonPersistentPreRun()
	}
	rootCmd.AddCommand(daemonCmd)
}

func execDaemonPersistentPreRun() {
	err := utils.CheckPerm(constants.SpmHome)
	if err != nil {
		log.Fatal(err)
	}
}

func execDaemonCmd(cmd *cobra.Command, args []string) {
	if isDaemonRunning() {
		fmt.Println("Spm supervisor daemon is running. Don't start again.")
		return
	}

	fmt.Printf("Supervisor is not running. Starting supervisor...\n\n")

	sv := supervisor.NewSupervisor()
	sv.Daemon()
}
