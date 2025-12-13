package supervisor

import "spm/pkg/codec"

func (se *SpmSession) dispatch(msg *codec.ActionMsg) codec.ResponseCtl {
	// 处理业务逻辑
	var res *codec.ResponseMsg
	var result codec.ResponseCtl

	switch msg.Action {
	case codec.ActionKill, codec.ActionShutdown:
		{
			// 先准备响应消息
			res = &codec.ResponseMsg{
				Code:    200,
				Message: "Shutdown prepared",
			}
			result = codec.ResponseShutdown

			// 执行优雅关闭
			se.sv.Shutdown()
		}
	case codec.ActionLog:
		res = &codec.ResponseMsg{
			Code:    405,
			Message: "Feature not implemented",
		}
		result = codec.ResponseMsgErr
	case codec.ActionDump:
		res, result = se.doDump()
	case codec.ActionLoad:
		res, result = se.doLoad()
	case codec.ActionRun:
		res = se.doRun(msg)
		result = codec.ResponseNormal
	case codec.ActionReload:
		res = se.doReload(msg)
		result = codec.ResponseReload
	default:
		res = se.doAction(msg)
		result = codec.ResponseNormal
	}

	return se.sendResponse(res, result)
}
