// Package supervisor 提供进程监督管理核心功能
//
// 本模块负责：
// - Supervisor 核心结构定义
// - 进程和项目表管理
// - 信号处理和生命周期控制
// - 进程操作（启动、停止、重启、状态查询）
// - 批量操作和配置重载
//
// 依赖：
// - pkg/config: 配置管理
// - pkg/logger: 日志记录
// - pkg/utils: 工具函数
//
// 架构说明：
//
//	Supervisor 采用表驱动设计，维护两个核心表：
//	1. ProjectTable：项目表，按项目组织进程
//	2. ProcTable：进程表，管理所有进程实例
//
// 文件组织：
//   - supervisor.go：核心结构定义
//   - tables.go：进程表管理
//   - operations.go：Start/Stop/Restart 操作
//   - batch.go：批量操作
//   - daemon.go：Daemon 和 Shutdown
//   - reload.go：配置重载
//   - app.go：应用/项目管理
//
// 使用示例：
//
//	sv := supervisor.NewSupervisor()
//	sv.Daemon()  // 以守护进程模式运行
//
// 创建时间: 2025-12-06
// 最后修改: 2025-12-06
package supervisor

import (
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"spm/pkg/logger"
	"spm/pkg/utils"

	"go.uber.org/zap"
)

var maxCpus = runtime.NumCPU()

// Supervisor 是管理维护进程组的核心控制器
//
// 职责：
// - 管理多个项目和进程
// - 处理进程生命周期（启动、停止、重启）
// - 监听系统信号并优雅关闭
// - 提供进程状态查询
//
// 线程安全：
// - 使用 RWMutex 保护内部状态
// - 所有公开方法都是并发安全的
//
// 字段说明：
//
//	AfterStart: 启动后回调函数（仅前台模式）
//	StartedAt: Supervisor 启动时间
//	Pid: Supervisor 进程 PID
//	mu: 读写锁，保护内部状态
//	logger: 日志记录器
//	projectTable: 项目表，管理所有项目
//	procTable: 进程表，管理所有进程
type Supervisor struct {
	AfterStart func()    // 启动后回调函数
	StartedAt  time.Time // 启动时间
	Pid        int       // Supervisor 进程 PID

	mu           sync.RWMutex       // 读写锁
	logger       *zap.SugaredLogger // 日志记录器
	projectTable *ProjectTable      // 项目表
	procTable    *ProcTable         // 进程表
}

// NewSupervisor 创建新的 Supervisor 实例
//
// 返回：
//
//	*Supervisor: 已初始化的 Supervisor 实例
//
// 初始化内容：
//  1. 注册系统信号监听（SIGINT, SIGTERM, SIGQUIT）
//  2. 初始化空的项目表和进程表
//  3. 设置当前时间为启动时间
//  4. 设置当前进程 PID
//  5. 初始化日志记录器
//
// 注意事项：
//   - 信号监听会在后台持续运行
//   - 使用 utils.StopChan 接收终止信号
//   - 默认 AfterStart 回调为空函数
//
// 示例：
//
//	sv := NewSupervisor()
//	defer sv.Shutdown()
//
//	// 自定义启动后回调
//	sv.AfterStart = func() {
//	    fmt.Println("Supervisor 已启动")
//	}
//
//	sv.Daemon()  // 阻塞运行
func NewSupervisor() *Supervisor {
	signal.Notify(utils.StopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	return &Supervisor{
		AfterStart: func() {},
		StartedAt:  time.Now(),
		Pid:        utils.SupervisorPid,
		logger:     logger.Logging("supervisor"),
		projectTable: &ProjectTable{
			table: make(map[string]*Project),
		},
		procTable: &ProcTable{
			table: make(map[string]*Process),
		},
	}
}
