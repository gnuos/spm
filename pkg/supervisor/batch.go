// Package supervisor 提供批量操作功能
package supervisor

import (
	"slices"
)

// BatchDo 批量执行进程操作
//
// 参数：
//
//	toDo: 操作类型（ActionStart/ActionStop/ActionRestart/ActionStatus）
//	opt: Procfile 配置选项
//	procs: 进程名列表，["*"] 表示所有进程
//
// 返回：
//
//	[]*ProcInfo: 操作结果列表
//
// 操作类型：
//   - ActionStart: 启动进程
//   - ActionStop: 停止进程
//   - ActionRestart: 重启进程
//   - ActionStatus: 查询状态
//
// 注意事项：
//  1. 会先调用 UpdateApp(true, opt) 确保进程已注册
//  2. 如果项目不存在，返回 nil
//  3. 支持通配符 "*" 匹配所有进程
//
// 错误处理：
//
//	项目不存在时记录错误日志并返回 nil
//
// 示例：
//
//	infos := sv.BatchDo(ActionStart, opt, []string{"*"})
//	infos := sv.BatchDo(ActionStop, opt, []string{"web-server", "worker"})
//
// 创建时间: 2025-12-06
func (sv *Supervisor) BatchDo(toDo ActionCtl, opt *ProcfileOption, procs []string) []*ProcInfo {
	var doFn func(string) *Process
	var doMany func(string) []*Process

	proj, _ := sv.UpdateApp(true, opt)
	if proj == nil {
		sv.logger.Errorf("Cannot find project in work directory %s", opt.WorkDir)
		return nil
	}

	switch toDo {
	case ActionStop:
		doFn = sv.Stop
		doMany = sv.StopAll
	case ActionStart:
		doFn = sv.Start
		doMany = sv.StartAll
	case ActionRestart:
		doFn = sv.Restart
		doMany = sv.RestartAll
	case ActionStatus:
		doFn = sv.Status
		doMany = sv.StatusAll
	}

	var pInfo = make([]*ProcInfo, 0)
	if slices.Contains(procs, "*") {
		completed := doMany("*")

		for _, p := range completed {
			pInfo = append(pInfo, &ProcInfo{
				Pid:     p.Pid,
				Name:    p.FullName,
				StartAt: p.StartAt.UnixMilli(),
				StopAt:  p.StopAt.UnixMilli(),
				Status:  p.State,
			})
		}
	} else {
		for _, name := range procs {
			p := doFn(name)
			if p != nil {
				pInfo = append(pInfo, &ProcInfo{
					Pid:     p.Pid,
					Name:    p.FullName,
					StartAt: p.StartAt.UnixMilli(),
					StopAt:  p.StopAt.UnixMilli(),
					Status:  p.State,
				})
			}
		}
	}

	return pInfo
}
