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

	"spm/pkg/config"
	"spm/pkg/logger"

	"github.com/ugorji/go/codec"
	"go.uber.org/zap"
)

type spmSocket struct {
	conn net.Conn
}

func (s *spmSocket) Recv(l uint64) ([]byte, error) {
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

func (s *spmSocket) Send(v []byte) error {
	_, e := s.conn.Write(v)
	return e
}

func (s *spmSocket) Close() error {
	return s.conn.Close()
}

type SpmSession struct {
	sv     *Supervisor
	sock   *spmSocket
	logger *zap.SugaredLogger
}

func NewSession(s *Supervisor, c net.Conn) *SpmSession {
	return &SpmSession{
		sv: s,
		sock: &spmSocket{
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
//	*ResponseMsg: 错误响应消息
//	ResponseCtl: 错误响应控制类型
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
func (se *SpmSession) errorResponse(err error) (*ResponseMsg, ResponseCtl) {
	se.logger.Error(err)
	return &ResponseMsg{
		Code:    500,
		Message: err.Error(),
	}, ResponseMsgErr
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
//	ResponseCtl: 如果发送成功返回传入的result，失败返回ResponseMsgErr
//
// 功能：
//  1. 编码响应消息
//  2. 发送消息长度
//  3. 发送消息内容
//  4. 统一处理发送过程中的错误
func (se *SpmSession) sendResponse(res *ResponseMsg, result ResponseCtl) ResponseCtl {
	buf, err := encodeData(res)
	if err != nil {
		se.logger.Error(err)
		return ResponseMsgErr
	}

	size := make([]byte, strconv.IntSize)
	binary.BigEndian.PutUint64(size, uint64(len(buf)))

	if err = se.sock.Send(size); err != nil {
		se.logger.Error(err)
		return ResponseMsgErr
	}

	if err = se.sock.Send(buf); err != nil {
		se.logger.Error(err)
		return ResponseMsgErr
	}

	return result
}

func (se *SpmSession) Handle() ResponseCtl {
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

	msg, err := decodeData[ActionMsg](buf)
	if err != nil {
		res, result := se.errorResponse(err)
		return se.sendResponse(res, result)
	}

	// 处理业务逻辑
	var res *ResponseMsg
	var result ResponseCtl

	switch msg.Action {
	case ActionKill, ActionShutdown:
		{
			// 先准备响应消息
			res = &ResponseMsg{
				Code:    200,
				Message: "Shutdown prepared",
			}
			result = ResponseShutdown

			// 执行优雅关闭
			se.sv.Shutdown()
		}
	case ActionLog:
		res = &ResponseMsg{
			Code:    404,
			Message: "Feature not implemented",
		}
		result = ResponseMsgErr
	case ActionRun:
		res = se.doRun(msg)
		result = ResponseNormal
	case ActionReload:
		res = se.doReload(msg)
		result = ResponseReload
	default:
		res = se.doAction(msg)
		result = ResponseNormal
	}

	return se.sendResponse(res, result)
}

func (se *SpmSession) doReload(msg *ActionMsg) *ResponseMsg {
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
			return &ResponseMsg{
				Code:    500,
				Message: "Reload failed",
			}
		} else {
			changedTotal = append(changedTotal, changed...)
		}
	}

	return &ResponseMsg{
		Code:      200,
		Message:   "Reload successfully",
		Processes: se.sv.Reload(changedTotal),
	}
}

func (se *SpmSession) doRun(msg *ActionMsg) *ResponseMsg {
	var exe string
	var args = make([]string, 0)

	exe = msg.CmdLine[0]

	exePath, err := exec.LookPath(exe)
	if err != nil {
		return &ResponseMsg{
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
		return &ResponseMsg{
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

		cmd: args,
	}

	// 第一遍注册进程
	_, _ = se.sv.UpdateApp(true, procOpts)

	// 第二遍reload进程表
	_, _ = se.sv.UpdateApp(false, procOpts)

	// 运行单个的进程
	infos := se.sv.BatchDo(ActionStart, procOpts, []string{fmt.Sprintf("%s::%s", appName, procName)})

	return &ResponseMsg{
		Code:      200,
		Message:   actionResponse[msg.Action],
		Processes: infos,
	}
}

func (se *SpmSession) doAction(msg *ActionMsg) *ResponseMsg {
	names := msg.Processes
	var origProcs []string

	if names == "*" {
		origProcs = []string{"*"}
	} else if strings.Contains(names, ";") {
		origProcs = strings.Split(names, ";")
	} else {
		origProcs = []string{names}
	}

	var localProcs = make([]string, 0)
	var infos = make([]*ProcInfo, 0)
	var procMap = make(map[string][]string)

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
			return &ResponseMsg{
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

	return &ResponseMsg{
		Code:      200,
		Message:   actionResponse[msg.Action],
		Processes: infos,
	}
}

func decodeData[T any](data []byte) (*T, error) {
	var msg = new(T)
	var mh codec.MsgpackHandle

	mh.StructToArray = true

	decoder := codec.NewDecoderBytes(data, &mh)
	err := decoder.Decode(msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func encodeData[T any](v *T) ([]byte, error) {
	var buf []byte
	var mh codec.MsgpackHandle

	mh.StructToArray = true

	encoder := codec.NewEncoderBytes(&buf, &mh)
	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
