// Package supervisor 提供配置重载功能
package supervisor

import (
	"spm/pkg/config"
	"spm/pkg/utils"
)

// Reload 重新加载配置并返回变更的进程信息
//
// 参数：
//
//	changed: 发生变更的进程列表
//
// 返回：
//
//	[]*ProcInfo: 进程信息列表
//
// 功能：
//  1. 重新加载配置文件
//  2. 将变更的进程信息转换为 ProcInfo 格式
//
// 注意事项：
//  1. 配置文件路径来自 utils.GlobalConfigFile
//  2. 仅转换进程信息，不执行实际的启动/停止操作
//
// 示例：
//
//	infos := sv.Reload(changedProcs)
//	for _, info := range infos {
//	    fmt.Printf("进程 %s 已变更\n", info.Name)
//	}
//
// 创建时间: 2025-12-06
func (sv *Supervisor) Reload(changed []*Process) []*ProcInfo {
	sv.logger.Info("Reloading configuration")
	config.SetConfig(utils.GlobalConfigFile)

	pInfo := make([]*ProcInfo, 0)

	if len(changed) > 0 {
		for _, p := range changed {
			pInfo = append(pInfo, &ProcInfo{
				Pid:     p.Pid,
				Name:    p.FullName,
				StartAt: p.StartAt.UnixMilli(),
				StopAt:  p.StopAt.UnixMilli(),
				Status:  p.State,
			})
		}
	}

	return pInfo
}
