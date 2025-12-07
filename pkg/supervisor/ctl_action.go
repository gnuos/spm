package supervisor

type ActionCtl int

const (
	ActionRun ActionCtl = iota
	ActionLog
	ActionKill
	ActionStart
	ActionStop
	ActionStatus
	ActionRestart
	ActionShutdown
	ActionReload
)

var actionResponse = map[ActionCtl]string{
	ActionRun:     "Run command successfully",
	ActionStart:   "Start processes successfully",
	ActionStop:    "Stop processes successfully",
	ActionStatus:  "Check processes stattus successfully",
	ActionRestart: "Restart processes successfully",
}

type ActionMsg struct {
	Action    ActionCtl
	WorkDir   string
	Procfile  string
	Projects  string
	Processes string
	CmdLine   []string
}
