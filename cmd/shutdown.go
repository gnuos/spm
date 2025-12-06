package cmd

import (
	"fmt"
	"log"
	"os"
	"spm/pkg/config"
	"spm/pkg/supervisor"
	"spm/pkg/utils"
	"syscall"

	"github.com/spf13/cobra"
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Stop supervisor",
	Run:   execShutdownCmd,
}

func init() {
	shutdownCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootCmd.PersistentPreRun(cmd, args)
		execShutdownPersistentPreRun()
	}
	rootCmd.AddCommand(shutdownCmd)
}

func execShutdownPersistentPreRun() {
	requireDaemonRunning()
}

func execShutdownCmd(cmd *cobra.Command, args []string) {
	msg.Action = supervisor.ActionShutdown

	_ = supervisor.ClientRun(msg)

	spid, err := utils.ReadPid(config.GetConfig().PidFile)
	if err != nil {
		log.Fatal(err)
	}

	if spid > 0 {
		err = syscall.Kill(spid, syscall.SIGQUIT)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err.Error())
			return
		}
	}

	fmt.Println("Supervisor service has been stopped.")
}
