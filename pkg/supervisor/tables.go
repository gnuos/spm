// Package supervisor 提供进程表管理功能
package supervisor

import (
	"iter"
	"sync"

	orderedmap "github.com/gnuos/omap"
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

	table *orderedmap.OrderedMap[string, *Process]
}

func NewProcTable() *ProcTable {
	return &ProcTable{
		table: orderedmap.New[string, *Process](),
	}
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
func (pt *ProcTable) Get(name string) (*Process, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.table.Get(name)
}

func (pt *ProcTable) Set(name string, p *Process) (*Process, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.table.Set(name, p)
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

	if _, ok := pt.table.Get(name); ok {
		return false
	}

	pt.table.Set(name, proc)

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

	_, ok := pt.table.Delete(name)

	return ok
}

// All returns an iterator over key-value pairs from m.
// The ordering will be oldest to newest, based on when a key was first set.
func (pt *ProcTable) All() iter.Seq2[string, *Process] {
	return pt.table.FromOldest()
}

// Backward returns an iterator over key-value pairs from m in reverse.
// The ordering will be newest to oldest, based on when a key was first set.
func (pt *ProcTable) Backward() iter.Seq2[string, *Process] {
	return pt.table.FromNewest()
}

// Keys returns an iterator over keys in m.
// The ordering will be oldest to newest, based on when a key was first set.
func (pt *ProcTable) Keys() iter.Seq[string] {
	return pt.table.KeysFromOldest()
}

// Values returns an iterator over values in m.
// The ordering will be oldest to newest, based on when a key was first set.
func (pt *ProcTable) Values() iter.Seq[*Process] {
	return pt.table.ValuesFromOldest()
}

type ProcList struct {
	mu sync.Mutex

	index []int          // 全局进程序号
	place map[string]int // 存放进程序号的位置
	table map[int]string // 存放进程表
}

func NewProcList() *ProcList {
	return &ProcList{
		index: make([]int, 0),
		place: make(map[string]int),
		table: make(map[int]string),
	}
}

func (pl *ProcList) Index(name string) int {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if i, ok := pl.place[name]; ok {
		return i
	}

	return -1
}

func (pl *ProcList) Get(i int) (string, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	value, ok := pl.table[i]

	return value, ok
}

func (pl *ProcList) Add(name string) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if _, ok := pl.place[name]; ok {
		return false
	}

	last := -1
	if len(pl.index) > 0 {
		last = pl.index[len(pl.index)-1]
	}

	pl.index = append(pl.index, last+1)
	pl.table[last+1] = name
	pl.place[name] = last + 1

	return true
}

func (pl *ProcList) Del(name string) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	i, ok := pl.place[name]
	if !ok {
		return false
	}

	delete(pl.table, i)
	delete(pl.place, name)

	return true
}

func (pl *ProcList) All() []string {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	all := make([]string, 0)

	for name := range pl.place {
		all = append(all, name)
	}

	return all
}
