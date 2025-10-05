package priorityFee

type MaxStrategy struct {
	IPriorityFeeStrategy
}

func (p *MaxStrategy) Calculate(samples []SolanaPriorityFeeResponse) uint64 {
	if len(samples) == 0 {
		return 0
	}
	runningMaxFee := uint64(0)
	for _, b := range samples {
		if runningMaxFee < b.PrioritizationFee {
			runningMaxFee = b.PrioritizationFee
		}
	}
	return runningMaxFee
}
