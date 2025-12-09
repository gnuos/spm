package codec

import "time"

type ResponseCtl int

const (
	ResponseNormal ResponseCtl = iota
	ResponseShutdown
	ResponseReload
	ResponseMsgErr
)

type ProcInfo struct {
	Pid     int          `json:"pid"`
	Name    string       `json:"name"`
	Project string       `json:"project"`
	StartAt time.Time    `json:"start_at"`
	StopAt  time.Time    `json:"stop_at"`
	Status  ProcessState `json:"status"`
}

type ResponseMsg struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Processes []*ProcInfo `json:"processes"`
}
