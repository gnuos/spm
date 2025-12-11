// Package supervisor 提供进程操作功能
package supervisor

import (
	"spm/pkg/codec"
	"strings"
)

// Status 获取单个进程的状态
//
// 参数：
//
//	name: 完整进程名（格式：appName::processName）
//
// 返回：
//
//	*Process: 进程实例，不存在时返回 nil
//
// 注意事项：
//  1. 这是一个只读查询方法，不会修改进程状态
//  2. 进程状态的维护由 Start/Stop 方法负责
//
// 线程安全：使用读锁保护，支持并发查询
//
// 示例：
//
//	proc := sv.Status("myapp::web-server")
//	if proc != nil {
//	    fmt.Println(proc.State)
//	}
//
// 创建时间: 2025-12-06
func (sv *Supervisor) Status(p *Process) *Process {
	return p
}

// forEachProcess 对指定范围的进程执行操作
//
// 参数：
//
//	appName: 项目名称，"*" 表示所有项目
//	operation: 对单个进程执行的操作函数
//
// 返回：
//
//	[]*Process: 操作成功的进程列表
//
// 说明：
//
//	提取公共的进程迭代逻辑，避免代码重复
func (sv *Supervisor) forEachProcess(appName string, operation func(*Process) *Process) []*Process {
	procs := make([]*Process, 0)

	if appName != "*" {
		proj := sv.projectTable.Get(appName)
		if proj == nil {
			return procs
		}

		for _, proc := range proj.GetProcs() {
			if p := operation(proc); p != nil {
				procs = append(procs, p)
			}
		}
	} else {
		for _, name := range sv.procList.All() {
			proc := sv.GetProcByName(name)
			if p := operation(proc); p != nil {
				procs = append(procs, p)
			}
		}
	}

	return procs
}

// StatusAll 获取项目下所有进程的状态
//
// 参数：
//
//	appName: 项目名称，"*" 表示所有项目
//
// 返回：
//
//	[]*Process: 进程列表
//
// 示例：
//
//	procs := sv.StatusAll("myapp")
//	procs := sv.StatusAll("*")  // 所有进程
func (sv *Supervisor) StatusAll(appName string) []*Process {
	return sv.forEachProcess(appName, sv.Status)
}

// Start 启动单个进程
//
// 参数：
//
//	name: 完整进程名
//
// 返回：
//
//	*Process: 启动成功返回进程实例，失败返回 nil
//
// 注意事项：
//  1. 如果进程已在运行，记录警告但返回成功
//  2. 启动后会更新项目表中的状态
//
// 示例：
//
//	proc := sv.Start("myapp::web-server")
//	if proc != nil {
//	    fmt.Printf("进程已启动，PID: %d\n", proc.Pid)
//	}
func (sv *Supervisor) Start(p *Process) *Process {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	appName := strings.Split(p.FullName, "::")[0]
	proj := sv.projectTable.Get(appName)

	if p.IsRunning() {
		p.logger.Warnf("%s already running with PID %d", p.FullName, p.Pid)

		proj.SetState(p.Name, true)

		// 对于重复执行Start的进程，不修改进程表的情况下，返回进程状态信息
		// 结构对齐ProcInfo
		return &Process{
			Pid:      p.Pid,
			FullName: p.FullName,
			StartAt:  p.StartAt,
			StopAt:   p.StopAt,
			State:    codec.ProcessStarted,
		}
	}

	state := p.Start()
	proj.SetState(p.Name, state)

	if state {
		return p
	} else {
		return &Process{
			Pid:      p.Pid,
			FullName: p.FullName,
			StartAt:  p.StartAt,
			StopAt:   p.StopAt,
			State:    codec.ProcessFailed,
		}
	}
}

// StartAll 启动项目下所有进程
//
// 参数：
//
//	appName: 项目名称，"*" 表示所有项目
//
// 返回：
//
//	[]*Process: 已启动的进程列表
//
// 示例：
//
//	procs := sv.StartAll("myapp")
//	fmt.Printf("启动了 %d 个进程\n", len(procs))
func (sv *Supervisor) StartAll(appName string) []*Process {
	return sv.forEachProcess(appName, sv.Start)
}

// Stop 停止单个进程
//
// 参数：
//
//	name: 完整进程名
//
// 返回：
//
//	*Process: 停止成功返回进程实例，失败返回 nil
//
// 注意事项：
//  1. 如果进程已停止，直接返回成功
//  2. 停止后会更新项目表中的状态
//
// 示例：
//
//	proc := sv.Stop("myapp::web-server")
//	if proc != nil {
//	    fmt.Printf("进程已停止：%s\n", proc.FullName)
//	}
func (sv *Supervisor) Stop(p *Process) *Process {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	appName := strings.Split(p.FullName, "::")[0]
	proj := sv.projectTable.Get(appName)

	if p.State == codec.ProcessRunning && proj.GetState(p.Name) {
		if p.Stop() {
			proj.SetState(p.Name, false)

			return p
		}
	}

	if p.State == codec.ProcessStopped {
		p.logger.Warnf("%s stopped already", p.FullName)
		proj.SetState(p.Name, false)
		return p
	}

	return &Process{
		Pid:      p.Pid,
		FullName: p.FullName,
		StartAt:  p.StartAt,
		StopAt:   p.StopAt,
		State:    p.State,
	}
}

// StopAll 停止项目下所有进程
//
// 参数：
//
//	appName: 项目名称，"*" 表示所有项目
//
// 返回：
//
//	[]*Process: 已停止的进程列表
//
// 注意事项：
//
//	对于特定项目，只停止当前运行中的进程
//
// 示例：
//
//	procs := sv.StopAll("myapp")
//	fmt.Printf("停止了 %d 个进程\n", len(procs))
func (sv *Supervisor) StopAll(appName string) []*Process {
	// 对于特定项目，需要检查进程状态，只停止运行中的进程
	if appName != "*" {
		proj := sv.projectTable.Get(appName)
		if proj == nil {
			return make([]*Process, 0)
		}

		return sv.forEachProcess(appName, func(p *Process) *Process {
			if proj.GetState(p.Name) {
				return sv.Stop(p)
			}

			return nil
		})
	} else {
		// 对于所有项目，直接调用 Stop
		return sv.forEachProcess(appName, func(p *Process) *Process {
			if p.State != codec.ProcessStopped {
				return sv.Stop(p)
			}
			return nil
		})
	}
}

// Restart 重启单个进程
//
// 参数：
//
//	name: 完整进程名
//
// 返回：
//
//	*Process: 重启成功返回进程实例，失败返回 nil
//
// 实现：
//
//	先调用 Stop 停止进程，再调用 Start 启动进程
//
// 示例：
//
//	proc := sv.Restart("myapp::web-server")
func (sv *Supervisor) Restart(p *Process) *Process {
	sv.Stop(p)
	return sv.Start(p)
}

// RestartAll 重启项目下所有进程
//
// 参数：
//
//	appName: 项目名称，"*" 表示所有项目
//
// 返回：
//
//	[]*Process: 已重启的进程列表
//
// 实现：
//
//	先调用 StopAll 停止所有进程，再调用 StartAll 启动所有进程
//
// 示例：
//
//	procs := sv.RestartAll("myapp")
func (sv *Supervisor) RestartAll(appName string) []*Process {
	sv.StopAll(appName)
	return sv.StartAll(appName)
}
