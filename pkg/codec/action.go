package codec

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

var ActionResponse = map[ActionCtl]string{
	ActionRun:     "Run command successfully",
	ActionStart:   "Start processes successfully",
	ActionStop:    "Stop processes successfully",
	ActionStatus:  "Check processes status successfully",
	ActionRestart: "Restart processes successfully",
}

type ActionMsg struct {
	Action    ActionCtl `cbor:""`
	WorkDir   string    `cbor:""`
	Procfile  string    `cbor:""`
	Projects  string    `cbor:",omitempty"`
	Processes string    `cbor:",omitempty"`
	CmdLine   []string  `cbor:",omitempty"`
}
