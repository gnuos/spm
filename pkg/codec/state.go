package codec

type ProcessState string

const (
	ProcessStarted  ProcessState = "Started"
	ProcessNotfound ProcessState = "NotFound"
	ProcessUnknown  ProcessState = "Unknown"
	ProcessStopped  ProcessState = "Stopped"
	ProcessStopping ProcessState = "Stopping"
	ProcessRunning  ProcessState = "Running"
	ProcessStandby  ProcessState = "Standby"
	ProcessFailed   ProcessState = "Failed"
)
