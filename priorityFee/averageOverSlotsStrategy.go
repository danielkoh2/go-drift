package priorityFee

type AverageOverSlotsStrategy struct {
	IPriorityFeeStrategy
}

func (p *AverageOverSlotsStrategy) Calculate(samples []SolanaPriorityFeeResponse) uint64 {
	if len(samples) == 0 {
		return 0
	}
	runningSumFees := uint64(0)
	for _, sample := range samples {
		runningSumFees += sample.PrioritizationFee
	}
	return runningSumFees / uint64(len(samples))
}
