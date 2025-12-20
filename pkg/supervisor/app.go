// Package supervisor 提供应用/项目管理功能
package supervisor

import (
	"fmt"
	"spm/pkg/codec"
)

// UpdateApp 更新或注册项目及其进程
//
// 参数：
//
//	force: 是否强制注册（true 时注册新项目，false 时更新现有项目）
//	procOpts: Procfile 配置选项
//
// 返回：
//
//	*Project: 项目实例
//	[]*Process: 新增的进程列表（仅 force=false 时返回）
//
// 工作模式：
//
//	force = true（注册模式）：
//	  - 仅当项目不存在时注册
//	  - 创建所有进程并添加到进程表
//	  - 初始化进程状态为 false（未运行）
//
//	force = false（更新模式）：
//	  - 对比新旧进程列表
//	  - 删除不再存在且未运行的进程
//	  - 添加新增的进程
//	  - 返回新增进程列表
//
// 注意事项：
//  1. 线程安全：使用 RWMutex 保护
//  2. 进程命名格式：appName::processName
//  3. 不会自动启动进程，仅注册
//
// 示例：
//
//	// 注册新项目
//	proj, _ := sv.UpdateApp(true, procOpts)
//
//	// 更新现有项目
//	_, newProcs := sv.UpdateApp(false, procOpts)
//	fmt.Printf("新增了 %d 个进程\n", len(newProcs))
//
// 创建时间: 2025-12-06
func (sv *Supervisor) UpdateApp(
	force bool,
	procOpts *ProcfileOption,
) (*Project, []*Process) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	newProj := CreateProject(procOpts)

	oldProj := sv.projectTable.Get(procOpts.AppName)
	if force {
		if oldProj == nil {
			if len(procOpts.Processes) == 0 || procOpts.WorkDir == "" {
				return nil, nil
			}

			_ = sv.projectTable.Set(procOpts.AppName, newProj)

			for name, opt := range procOpts.Processes {
				proc := newProj.Register(name, opt)
				sv.procList.Add(proc.FullName)
			}

			return newProj, nil
		}
	} else {
		if oldProj != nil {
			// 记录新增的进程信息
			pList := make([]*Process, 0)
			oldProcList := oldProj.GetProcNames()

			for _, name := range oldProcList {
				if !newProj.IsExist(name) && !oldProj.GetState(name) {
					fullName := fmt.Sprintf("%s::%s", oldProj.Name, name)
					oldProj.Unset(name)
					_ = oldProj.procTable.Del(name)
					_ = sv.procList.Del(fullName)
				}
			}

			for name, opt := range procOpts.Processes {
				fullName := fmt.Sprintf("%s::%s", newProj.Name, name)
				exist := sv.GetProcByName(fullName)

				if exist != nil && exist.State != codec.ProcessNotfound {
					continue
				}

				proc := oldProj.Register(name, opt)
				sv.procList.Add(fullName)

				pList = append(pList, proc)
			}

			return oldProj, pList
		}
	}

	return oldProj, nil
}
