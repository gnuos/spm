// Package client 提供进程管理客户端的高级封装
//
// 本包封装了与 supervisor 通信的底层细节，为 cmd 层提供简洁的 API。
// 通过引入这一抽象层，降低了 cmd 包与 supervisor 包之间的耦合度。
//
// 设计目标：
//  1. 隔离 cmd 层与 supervisor 内部实现细节
//  2. 消除重复的消息构造逻辑
//  3. 提供易于测试和维护的接口
//  4. 为未来扩展（如 HTTP API）预留空间
package client

import (
	"strings"

	"spm/pkg/codec"
	"spm/pkg/supervisor"
)

// Start 启动一个或多个进程
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//	processes: 进程名列表，如果为空则启动所有进程
//
// 返回：
//
//	[]*supervisor.ProcInfo: 启动的进程信息列表
//	  - 如果没有进程启动，返回 nil
//	  - 每个 ProcInfo 包含进程的 PID、名称、启动时间等信息
//
// 使用示例：
//
//	// 启动所有进程
//	infos := client.Start("/path/to/workdir", "Procfile")
//
//	// 启动指定进程
//	infos := client.Start("/path/to/workdir", "Procfile", "web", "worker")
//
// 注意事项：
//   - 此函数通过 Unix Socket 与 supervisor daemon 通信
//   - 如果 daemon 未启动，将返回 nil 并在 stderr 输出错误
func Start(workDir, procfile string, processes ...string) []*codec.ProcInfo {
	msg := buildActionMsg(codec.ActionStart, workDir, procfile, processes)
	return supervisor.ClientRun(msg)
}

// Stop 停止一个或多个进程
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//	processes: 进程名列表，如果为空则停止所有进程
//
// 返回：
//
//	[]*supervisor.ProcInfo: 停止的进程信息列表
//	  - 如果没有进程停止，返回 nil
//	  - 每个 ProcInfo 包含进程的 PID、名称、停止时间等信息
//
// 使用示例：
//
//	// 停止所有进程
//	infos := client.Stop("/path/to/workdir", "Procfile")
//
//	// 停止指定进程
//	infos := client.Stop("/path/to/workdir", "Procfile", "web")
//
// 注意事项：
//   - 进程会收到 SIGTERM 信号进行优雅关闭
//   - 停止信号类型在 Procfile 中配置（默认为 TERM）
func Stop(workDir, procfile string, processes ...string) []*codec.ProcInfo {
	msg := buildActionMsg(codec.ActionStop, workDir, procfile, processes)
	return supervisor.ClientRun(msg)
}

// Restart 重启一个或多个进程
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//	processes: 进程名列表，如果为空则重启所有进程
//
// 返回：
//
//	[]*supervisor.ProcInfo: 重启的进程信息列表
//	  - 如果没有进程重启，返回 nil
//	  - 每个 ProcInfo 包含进程的 PID、名称、启动时间等信息
//
// 使用示例：
//
//	// 重启所有进程
//	infos := client.Restart("/path/to/workdir", "Procfile")
//
//	// 重启指定进程
//	infos := client.Restart("/path/to/workdir", "Procfile", "web", "worker")
//
// 注意事项：
//   - Restart = Stop + Start，会分配新的 PID
//   - 如果进程已经停止，则只执行 Start
func Restart(workDir, procfile string, processes ...string) []*codec.ProcInfo {
	msg := buildActionMsg(codec.ActionRestart, workDir, procfile, processes)
	return supervisor.ClientRun(msg)
}

// Status 查询一个或多个进程的状态
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//	processes: 进程名列表，如果为空则查询所有进程
//
// 返回：
//
//	[]*supervisor.ProcInfo: 进程状态信息列表
//	  - 如果没有找到进程，返回 nil
//	  - 每个 ProcInfo 包含进程的状态（Running/Stopped）
//
// 使用示例：
//
//	// 查询所有进程状态
//	infos := client.Status("/path/to/workdir", "Procfile")
//
//	// 查询指定进程状态
//	infos := client.Status("/path/to/workdir", "Procfile", "web")
//
// 注意事项：
//   - 此操作不会修改进程状态，只读取当前状态
func Status(workDir, procfile string, processes ...string) []*codec.ProcInfo {
	msg := buildActionMsg(codec.ActionStatus, workDir, procfile, processes)
	return supervisor.ClientRun(msg)
}

// Reload 重新加载配置并重启受影响的进程
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//
// 返回：
//
//	[]*supervisor.ProcInfo: 重新加载的进程信息列表
//	  - 如果没有配置变化，返回 nil
//	  - 只有配置发生变化的进程会被重启
//
// 使用示例：
//
//	infos := client.Reload("/path/to/workdir", "Procfile")
//
// 注意事项：
//   - 此操作会比较新旧配置，只重启发生变化的进程
//   - 未变化的进程保持运行，不受影响
func Reload(workDir, procfile string) []*codec.ProcInfo {
	msg := &codec.ActionMsg{
		Action:   codec.ActionReload,
		WorkDir:  workDir,
		Procfile: procfile,
	}
	return supervisor.ClientRun(msg)
}

// Shutdown 关闭 supervisor daemon
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//
// 返回：
//
//	[]*supervisor.ProcInfo: 始终返回 nil（关闭操作不返回进程信息）
//
// 使用示例：
//
//	client.Shutdown("/path/to/workdir", "Procfile")
//
// 注意事项：
//   - 此操作会停止所有管理的进程并关闭 supervisor daemon
//   - 操作是异步的，调用后 daemon 会在短时间内关闭
//   - 客户端可能会因为 daemon 关闭而收不到响应（这是正常的）
func Shutdown(workDir, procfile string) []*codec.ProcInfo {
	msg := &codec.ActionMsg{
		Action:   codec.ActionShutdown,
		WorkDir:  workDir,
		Procfile: procfile,
	}
	return supervisor.ClientRun(msg)
}

// Run 将一个命令作为进程运行
//
// 参数：
//
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//	cmdLine: 要执行的命令及其参数（例如：["ls", "-la"]）
//
// 返回：
//
//	[]*supervisor.ProcInfo: 运行的进程信息列表
//	  - 如果命令启动失败，返回 nil
//	  - 成功时返回包含一个元素的列表
//
// 使用示例：
//
//	// 运行 "ls -la" 命令
//	infos := client.Run("/path/to/workdir", "Procfile", []string{"ls", "-la"})
//
// 注意事项：
//   - 此命令会将临时命令注册为 supervisor 管理的进程
//   - 进程名称自动从可执行文件名提取
//   - 命令会在 workDir 目录下执行
func Run(workDir, procfile string, cmdLine []string) []*codec.ProcInfo {
	msg := &codec.ActionMsg{
		Action:   codec.ActionRun,
		WorkDir:  workDir,
		Procfile: procfile,
		CmdLine:  cmdLine,
	}
	return supervisor.ClientRun(msg)
}

func Dump(workDir, procfile string) []*codec.ProcInfo {
	msg := &codec.ActionMsg{
		Action:   codec.ActionDump,
		WorkDir:  workDir,
		Procfile: procfile,
	}
	return supervisor.ClientRun(msg)
}

func Load(workDir, procfile string) []*codec.ProcInfo {
	msg := &codec.ActionMsg{
		Action:   codec.ActionLoad,
		WorkDir:  workDir,
		Procfile: procfile,
	}
	return supervisor.ClientRun(msg)
}

// buildActionMsg 内部辅助函数，构建 ActionMsg 消息
//
// 功能：
//  1. 封装消息构造的通用逻辑
//  2. 处理进程列表的拼接规则（"*" 或 "proc1;proc2"）
//  3. 统一消息格式
//
// 参数：
//
//	action: 操作类型（ActionStart/ActionStop 等）
//	workDir: 工作目录路径
//	procfile: Procfile 配置文件路径
//	processes: 进程名列表
//
// 返回：
//
//	*supervisor.ActionMsg: 构造好的消息对象
//
// 进程列表规则：
//   - 空列表 []：转换为 "*"（所有进程）
//   - 单个进程 ["web"]：转换为 "web"
//   - 多个进程 ["web", "worker"]：转换为 "web;worker"
func buildActionMsg(action codec.ActionCtl, workDir, procfile string, processes []string) *codec.ActionMsg {
	// 处理进程列表：空数组表示所有进程
	procs := "*"
	if len(processes) == 1 {
		procs = processes[0]
	} else if len(processes) > 1 {
		procs = strings.Join(processes, ";")
	}

	return &codec.ActionMsg{
		Action:    action,
		WorkDir:   workDir,
		Procfile:  procfile,
		Processes: procs,
	}
}
