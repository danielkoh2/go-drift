package priorityFee

type AverageStrategy struct {
	IPriorityFeeStrategy
}

func (p *AverageStrategy) Calculate(samples []SolanaPriorityFeeResponse) uint64 {
	if len(samples) == 0 {
		return 0
	}
	runningSumFees := uint64(0)
	for _, b := range samples {
		runningSumFees += b.PrioritizationFee
	}
	return runningSumFees / uint64(len(samples))
}
