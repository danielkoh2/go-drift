package priorityFee

type MaxOverSlotsStrategy struct {
	IPriorityFeeStrategy
}

func (p *MaxOverSlotsStrategy) Calculate(samples []SolanaPriorityFeeResponse) uint64 {
	if len(samples) == 0 {
		return 0
	}
	runningMaxFee := uint64(0)
	for _, b := range samples {
		if runningMaxFee == 0 || runningMaxFee < b.PrioritizationFee {
			runningMaxFee = b.PrioritizationFee
		}
	}
	return runningMaxFee
}
