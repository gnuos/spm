// Package supervisor
package supervisor

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"spm/pkg/codec"
	"spm/pkg/config"
	"spm/pkg/logger"

	"github.com/fxamacker/cbor/v2"
	"go.uber.org/zap"
)

type rpcSocket struct {
	conn net.Conn
}

func (s *rpcSocket) Recv(l uint64) ([]byte, error) {
	buf := make([]byte, l)
	n, err := s.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return buf, io.EOF
	} else {
		return buf, nil
	}
}

func (s *rpcSocket) Send(v []byte) error {
	_, e := s.conn.Write(v)
	return e
}

func (s *rpcSocket) Close() error {
	return s.conn.Close()
}

type SpmSession struct {
	sv     *Supervisor
	sock   *rpcSocket
	logger *zap.SugaredLogger
}

func NewSession(s *Supervisor, c net.Conn) *SpmSession {
	return &SpmSession{
		sv: s,
		sock: &rpcSocket{
			conn: c,
		},
		logger: logger.Logging("spm-serv"),
	}
}

// errorResponse 创建错误响应消息的辅助函数
//
// 参数：
//
//	err: 错误对象
//
// 返回：
//
//	*codec.ResponseMsg: 错误响应消息
//	codec.ResponseCtl: 错误响应控制类型
//
// 功能：
//  1. 记录错误日志
//  2. 创建标准的500错误响应
//
// 使用示例：
//
//	if err != nil {
//	    res, result := se.errorResponse(err)
//	    return se.sendResponse(res, result)
//	}
func (se *SpmSession) errorResponse(err error) (*codec.ResponseMsg, codec.ResponseCtl) {
	se.logger.Error(err)
	return &codec.ResponseMsg{
		Code:    500,
		Message: err.Error(),
	}, codec.ResponseMsgErr
}

// sendResponse 发送响应消息到客户端
//
// 参数：
//
//	res: 响应消息对象
//	result: 响应控制类型
//
// 返回：
//
//	codec.ResponseCtl: 如果发送成功返回传入的result，失败返回ResponseMsgErr
//
// 功能：
//  1. 编码响应消息
//  2. 发送消息长度
//  3. 发送消息内容
//  4. 统一处理发送过程中的错误
func (se *SpmSession) sendResponse(res *codec.ResponseMsg, result codec.ResponseCtl) codec.ResponseCtl {
	encoder, err := codec.GetEncoder()
	if err != nil {
		se.logger.Error(err)
		return codec.ResponseMsgErr
	}

	buf, err := encoder.Marshal(res)
	if err != nil {
		se.logger.Error(err)
		return codec.ResponseMsgErr
	}

	size := make([]byte, strconv.IntSize)
	binary.BigEndian.PutUint64(size, uint64(len(buf)))

	if err = se.sock.Send(size); err != nil {
		se.logger.Error(err)
		return codec.ResponseMsgErr
	}

	if err = se.sock.Send(buf); err != nil {
		se.logger.Error(err)
		return codec.ResponseMsgErr
	}

	return result
}

func (se *SpmSession) Handle() codec.ResponseCtl {
	defer func() {
		_ = se.sock.Close()
	}()

	// 服务器端处理收到的指令

	// 先接收消息的字节数组长度
	buf, err := se.sock.Recv(strconv.IntSize)
	if err != nil {
		res, result := se.errorResponse(err)
		return se.sendResponse(res, result)
	}

	// 根据长度再接受ActionMsg消息
	msgLen := binary.BigEndian.Uint64(buf)
	buf, err = se.sock.Recv(msgLen)
	if err != nil {
		res, result := se.errorResponse(err)
		return se.sendResponse(res, result)
	}

	var msg = new(codec.ActionMsg)
	err = cbor.Unmarshal(buf, msg)
	if err != nil {
		res, result := se.errorResponse(err)
		return se.sendResponse(res, result)
	}

	return se.dispatch(msg)
}

func (se *SpmSession) doReload(msg *codec.ActionMsg) *codec.ResponseMsg {
	changedTotal := make([]*Process, 0)
	procOpts := make([]*ProcfileOption, 0)

	if msg.Projects != "" {
		if strings.Contains(msg.Projects, ";") {
			projects := strings.SplitSeq(msg.Projects, ";")
			for p := range projects {
				procOpts = append(procOpts, &ProcfileOption{AppName: p})
			}
		} else {
			procOpts = append(procOpts, &ProcfileOption{AppName: msg.Projects})
		}
	} else {
		if msg.WorkDir != "" && msg.Procfile != "" {
			opt, err := LoadProcfileOption(msg.WorkDir, msg.Procfile)
			if err != nil {
				se.logger.Error(err)
			} else {
				procOpts = append(procOpts, opt)
			}
		}
	}

	for _, opt := range procOpts {
		_, changed := se.sv.UpdateApp(false, opt)
		if changed == nil {
			se.logger.Errorf("Cannot find project %s.", opt.AppName)
			return &codec.ResponseMsg{
				Code:    500,
				Message: "Reload failed",
			}
		} else {
			changedTotal = append(changedTotal, changed...)
		}
	}

	return &codec.ResponseMsg{
		Code:      200,
		Message:   "Reload successfully",
		Processes: se.sv.Reload(changedTotal),
	}
}

func (se *SpmSession) doRun(msg *codec.ActionMsg) *codec.ResponseMsg {
	var exe string
	var args = make([]string, 0)

	exe = msg.CmdLine[0]

	exePath, err := exec.LookPath(exe)
	if err != nil {
		return &codec.ResponseMsg{
			Code:    500,
			Message: err.Error(),
		}
	}

	args = append(args, exePath)

	if len(msg.CmdLine) > 1 {
		args = append(args, msg.CmdLine[1:]...)
	}

	appName, err := GetAppName(msg.WorkDir)
	if err != nil {
		return &codec.ResponseMsg{
			Code:    500,
			Message: err.Error(),
		}
	}

	procName := filepath.Base(exePath)

	// 手工写项目的配置参数，用于手动将执行的命令注册为托管的进程
	procOpts := &ProcfileOption{
		AppName:   appName,
		WorkDir:   msg.WorkDir,
		Procfile:  msg.Procfile,
		Env:       make([]string, 0),
		Processes: make(map[string]*ProcessOption),
	}

	procOpts.Processes[procName] = &ProcessOption{
		Root:       msg.WorkDir,
		PidRoot:    config.GetRuntimeDir("/var"),
		LogRoot:    config.GetRuntimeDir("/var"),
		Env:        make([]string, 0),
		StopSignal: "TERM",
		NumProcs:   1,

		Cmd: args,
	}

	// 第一遍注册进程
	_, _ = se.sv.UpdateApp(true, procOpts)

	// 第二遍reload进程表
	_, _ = se.sv.UpdateApp(false, procOpts)

	// 运行单个的进程
	infos := se.sv.BatchDo(codec.ActionStart, procOpts, []string{fmt.Sprintf("%s::%s", appName, procName)})

	return &codec.ResponseMsg{
		Code:      200,
		Message:   codec.ActionResponse[msg.Action],
		Processes: infos,
	}
}

func (se *SpmSession) doAction(msg *codec.ActionMsg) *codec.ResponseMsg {
	names := msg.Processes
	var origProcs []string

	if names == "*" {
		origProcs = []string{"*"}
	} else if strings.Contains(names, ";") {
		origProcs = strings.Split(names, ";")
	} else {
		origProcs = []string{names}
	}

	localProcs := make([]string, 0)
	procMap := make(map[string][]string)
	infos := make([]*codec.ProcInfo, 0)

	for _, n := range origProcs {
		if strings.Contains(n, "::") {
			names := strings.Split(n, "::")
			appName := names[0]

			procMap[appName] = make([]string, 0)
			procMap[appName] = append(procMap[appName], n)
		} else {
			localProcs = append(localProcs, n)
		}
	}

	var procOpts *ProcfileOption
	var err error

	if len(localProcs) > 0 {
		procOpts, err = LoadProcfileOption(msg.WorkDir, msg.Procfile)
		if err != nil {
			se.logger.Error(err)
			return &codec.ResponseMsg{
				Code:    500,
				Message: "Load procfile options failed.",
			}
		}

		if localProcs[0] != "*" {
			for i, n := range localProcs {
				localProcs[i] = fmt.Sprintf("%s::%s", procOpts.AppName, n)
			}
		}

		procMap[procOpts.AppName] = localProcs
	}

	for name, procs := range procMap {
		var opt *ProcfileOption

		proj := se.sv.projectTable.Get(name)
		if proj == nil {
			opt = procOpts
		} else {
			opt = &ProcfileOption{AppName: name}
		}

		infos = append(infos, se.sv.BatchDo(msg.Action, opt, procs)...)
	}

	return &codec.ResponseMsg{
		Code:      200,
		Message:   codec.ActionResponse[msg.Action],
		Processes: infos,
	}
}
