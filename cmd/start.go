package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"spm/pkg/config"
	"spm/pkg/supervisor"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts processes and/or the supervisor",
	Run:   execStartCmd,
}

func init() {
	startCmd.PersistentFlags().BoolVarP(&config.ForegroundFlag, "foreground", "f", false, "Run the supervisor in the foreground")

	// start命令特殊处理：尝试启动daemon而不是要求daemon已运行
	setupCommandPreRun(startCmd, func() {
		if !config.ForegroundFlag {
			if isDaemonRunning() {
				return
			}

			if err := tryRunDaemon(); err != nil {
				log.Fatal(err)
			}

			time.Sleep(1 * time.Second)
		}
	})

	rootCmd.AddCommand(startCmd)
}

func execStartCmd(cmd *cobra.Command, args []string) {
	sendStartCmd := func(args []string) {
		msg.Action = supervisor.ActionStart
		msg.Processes = parseProcessArgs(args, ";")

		res := supervisor.ClientRun(msg)
		if res == nil {
			fmt.Println("No processes to start.")
			return
		}

		for _, proc := range res {
			fmt.Printf("[%s] Started %s\t[PID %d]\n", time.UnixMilli(proc.StartAt).Format(time.RFC3339), proc.Name, proc.Pid)
		}
	}

	if config.ForegroundFlag && !isDaemonRunning() {
		sv := supervisor.NewSupervisor()

		opt, err := supervisor.LoadProcfileOption(config.WorkDirFlag, config.ProcfileFlag)
		if err != nil {
			log.Fatal(err)
		}

		// 注册当前项目中的进程表
		proj, _ := sv.UpdateApp(true, opt)
		if proj == nil {
			log.Fatalf("Cannot find project in work directory %s", config.WorkDirFlag)
		}

		// 设置前台启动进程的回调方法
		sv.AfterStart = func() {
			sendStartCmd(args)
		}
		sv.Daemon()
	} else {
		sendStartCmd(args)
	}
}
