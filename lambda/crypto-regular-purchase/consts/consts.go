package consts

const (
	StartApplicationEventType = "start"
	StopApplicationEventType  = "stop"
	TargetTagKey              = "CryptoTradingScheduler"
	InTarget                  = "true"
	StopOnly                  = "stop"
	NameTagKey                = "Name"
)

type TargetStatus string

var (
	InTargetStatus    TargetStatus = "in_target"
	NotInTargetStatus TargetStatus = "not_in_target"
	NoStatusStatus    TargetStatus = "no_status"
)
