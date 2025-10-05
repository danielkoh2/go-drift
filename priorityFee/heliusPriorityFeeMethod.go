package priorityFee

type HeliusPriorityLevel string

const (
	HeliusPriorityLevelMin       HeliusPriorityLevel = "min"
	HeliusPriorityLevelLow       HeliusPriorityLevel = "low"
	HeliusPriorityLevelMedium    HeliusPriorityLevel = "medium"
	HeliusPriorityLevelHigh      HeliusPriorityLevel = "high"
	HeliusPriorityLevelVeryHigh  HeliusPriorityLevel = "veryHigh"
	HeliusPriorityLevelUnsafeMax HeliusPriorityLevel = "unsafeMax"
)

type HeliusPriorityFeeLevels map[HeliusPriorityLevel]uint64

type HeliusPriorityFeeResult struct {
	PriorityFeeEstimate uint64
	PriorityFeeLevels   HeliusPriorityFeeLevels
}
type HeliusPriorityFeeResponse struct {
	Jsonrpc string
	Result  HeliusPriorityFeeResult
	Id      string
}
