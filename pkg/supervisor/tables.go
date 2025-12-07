// Package supervisor 提供进程表管理功能
package supervisor

import (
	"sync"
)

// ProcTable 进程表，管理所有进程实例
//
// 职责：
// - 存储进程名到进程实例的映射
// - 提供线程安全的增删查操作
// - 支持遍历所有进程
//
// 线程安全：使用 RWMutex 保护并发访问
//
// 创建时间: 2025-12-06
type ProcTable struct {
	mu sync.RWMutex

	table map[string]*Process
}

// Get 根据进程名获取进程实例
//
// 参数：
//
//	name: 完整进程名（格式：appName::processName）
//
// 返回：
//
//	*Process: 进程实例，不存在时返回 nil
//
// 线程安全：使用读锁保护
//
// 示例：
//
//	proc := procTable.Get("myapp::web-server")
func (pt *ProcTable) Get(name string) *Process {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	p, ok := pt.table[name]
	if ok {
		return p
	}

	return nil
}

// Add 添加新进程到进程表
//
// 参数：
//
//	name: 完整进程名
//	proc: 进程实例
//
// 返回：
//
//	bool: 添加成功返回 true，进程已存在返回 false
//
// 线程安全：使用写锁保护
//
// 示例：
//
//	success := procTable.Add("myapp::web-server", proc)
func (pt *ProcTable) Add(name string, proc *Process) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, ok := pt.table[name]; ok {
		return false
	}

	pt.table[name] = proc

	return true
}

// Del 从进程表中删除进程
//
// 参数：
//
//	name: 完整进程名
//
// 返回：
//
//	bool: 删除成功返回 true，进程不存在返回 false
//
// 注意事项：
//
//	删除前会同步进程日志
//
// 线程安全：使用写锁保护
//
// 示例：
//
//	procTable.Del("myapp::web-server")
func (pt *ProcTable) Del(name string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	p, ok := pt.table[name]
	if !ok {
		return false
	}

	_ = p.logger.Sync()

	delete(pt.table, name)

	return true
}

// Iter 返回进程表的 map 引用（用于遍历）
//
// 返回：
//
//	map[string]*Process: 进程名到进程实例的映射
//
// 警告：
//
//	⚠️  返回的是内部 map 的引用，调用者遍历时应避免修改
//	⚠️  建议在调用时持有锁，或者返回副本
//
// TODO: 应该返回副本而不是引用，避免并发安全问题
//
// 示例：
//
//	procs := procTable.Iter()
//	for name, proc := range procs {
//	    fmt.Println(name, proc.State)
//	}
func (pt *ProcTable) Iter() map[string]*Process {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	return pt.table
}
