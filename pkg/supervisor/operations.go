// Package supervisor 提供进程操作功能
package supervisor

import (
	"fmt"
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
//  1. 会检查进程是否正在运行
//  2. 如果运行中，会更新项目表中的状态
//
// 示例：
//
//	proc := sv.Status("myapp::web-server")
//	if proc != nil {
//	    fmt.Println(proc.State)
//	}
//
// 创建时间: 2025-12-06
func (sv *Supervisor) Status(name string) *Process {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	p := sv.procTable.Get(name)
	if p == nil {
		return nil
	}

	if p.IsRunning() {
		appName := strings.Split(name, "::")[0]
		proj := sv.projectTable.Get(appName)
		proj.SetState(p.Name, true)
	}

	return p
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
func (sv *Supervisor) StatusAll(appName string) (procs []*Process) {
	procs = make([]*Process, 0)
	var proj *Project
	if appName != "*" {
		proj = sv.projectTable.Get(appName)
		if proj == nil {
			return
		}

		plist := proj.GetProcNames()
		for _, name := range plist {
			fullName := fmt.Sprintf("%s::%s", appName, name)
			p := sv.Status(fullName)
			if p != nil {
				procs = append(procs, p)
			}
		}
	} else {
		pt := sv.procTable.Iter()
		for name := range pt {
			p := sv.Status(name)
			procs = append(procs, p)
		}
	}

	return
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
func (sv *Supervisor) Start(name string) *Process {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	p := sv.procTable.Get(name)
	if p == nil {
		return nil
	}

	appName := strings.Split(name, "::")[0]
	proj := sv.projectTable.Get(appName)

	if p.IsRunning() {
		p.logger.Warnf("%s already running with PID %d", p.FullName, p.Pid)

		proj.SetState(p.Name, true)
		return p
	}

	state := p.Start()
	proj.SetState(p.Name, state)

	if state {
		return p
	} else {
		return nil
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
func (sv *Supervisor) StartAll(appName string) (procs []*Process) {
	procs = make([]*Process, 0)

	var proj *Project
	if appName != "*" {
		proj = sv.projectTable.Get(appName)
		if proj == nil {
			return
		}

		plist := proj.GetProcNames()
		for _, name := range plist {
			fullName := fmt.Sprintf("%s::%s", appName, name)
			p := sv.Start(fullName)
			if p != nil {
				procs = append(procs, p)
			}
		}
	} else {
		pt := sv.procTable.Iter()
		for name := range pt {
			p := sv.Start(name)
			procs = append(procs, p)
		}
	}

	return
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
func (sv *Supervisor) Stop(name string) *Process {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	p := sv.procTable.Get(name)
	if p == nil {
		return nil
	}

	appName := strings.Split(name, "::")[0]
	proj := sv.projectTable.Get(appName)

	if p.State == processStopped {
		p.logger.Infof("%s is stopped.", p.FullName)
		proj.SetState(p.Name, false)
		return p
	}

	if proj.GetState(p.Name) {
		if p.Stop() {
			proj.SetState(p.Name, false)
			return p
		}
	}

	return nil
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
// 示例：
//
//	procs := sv.StopAll("myapp")
//	fmt.Printf("停止了 %d 个进程\n", len(procs))
func (sv *Supervisor) StopAll(appName string) (procs []*Process) {
	procs = make([]*Process, 0)

	var proj *Project
	if appName != "*" {
		proj = sv.projectTable.Get(appName)
		if proj == nil {
			return
		}

		plist := proj.GetProcNames()
		for _, name := range plist {
			if proj.GetState(name) {
				fullName := fmt.Sprintf("%s::%s", appName, name)
				p := sv.Stop(fullName)
				if p != nil {
					procs = append(procs, p)
				}
			}
		}
	} else {
		pt := sv.procTable.Iter()
		for name := range pt {
			p := sv.Stop(name)
			procs = append(procs, p)
		}
	}

	return
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
func (sv *Supervisor) Restart(name string) *Process {
	sv.Stop(name)
	return sv.Start(name)
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
