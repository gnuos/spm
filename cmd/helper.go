package cmd

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"spm/pkg/utils"
	"spm/pkg/utils/constants"
)

func isDaemonRunning() bool {
	daemonPid, err := utils.ReadPid(constants.DaemonPidFilePath)
	if err != nil {
		return false
	}

	if daemonPid < 0 {
		return false
	}

	return isPidActive(daemonPid)
}

func isPidActive(p int) bool {
	_, err := syscall.Getpgid(p)

	return err == nil
}

func tryRunDaemon() error {
	var cmd *exec.Cmd
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := make([]string, 0)
	args = append(args, "daemon")
	args = append(args, os.Args[2:]...)

	cmd = exec.Command(exe, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	err = cmd.Start()

	return err
}

// parseProcessArgs 解析进程参数
//
// 参数：
//
//	args: 命令行参数列表
//	separator: 多个参数的分隔符（通常是 ";" 或 "|"）
//
// 返回：
//
//	string: 解析后的进程名字符串
//	  - 无参数时返回 "*" (所有进程)
//	  - 单个参数时返回该参数
//	  - 多个参数时用分隔符连接
//
// 使用示例：
//
//	procs := parseProcessArgs(args, ";")  // "web;worker"
//	procs := parseProcessArgs([]string{}, ";")  // "*"
//	procs := parseProcessArgs([]string{"web"}, ";")  // "web"
func parseProcessArgs(args []string, separator string) string {
	if len(args) == 0 {
		return "*"
	} else if len(args) == 1 {
		return args[0]
	}
	return strings.Join(args, separator)
}

// requireDaemonRunning 检查守护进程是否运行
//
// 功能：
//  1. 检查守护进程状态
//  2. 如果未运行，打印错误并退出程序
//
// 使用场景：
//
//	在需要守护进程运行的命令中（stop/restart/status/shutdown/reload）
//	调用此函数确保守护进程已启动
//
// 使用示例：
//
//	func execStopPersistentPreRun() {
//	    requireDaemonRunning()
//	}
func requireDaemonRunning() {
	if !isDaemonRunning() {
		log.Fatalln("ERROR: Supervisor has not started. Please check supervisor daemon.")
	}
}
