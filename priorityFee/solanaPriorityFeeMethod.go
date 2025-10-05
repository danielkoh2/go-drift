package priorityFee

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"slices"
)

type SolanaPriorityFeeResponse struct {
	Slot              uint64
	PrioritizationFee uint64
}

func FetchSolanaPriorityFee(connection *rpc.Client, lookbackDistance uint64, addresses solana.PublicKeySlice, percentile uint) []SolanaPriorityFeeResponse {

	response, err := GetRecentPrioritizationFeesEx(connection, context.TODO(), addresses, percentile)
	if err != nil {
		return []SolanaPriorityFeeResponse{}
	}
	if len(response) == 0 {
		return []SolanaPriorityFeeResponse{}
	}
	slices.SortFunc(response, func(a, b rpc.PriorizationFeeResult) int {
		if a.Slot < b.Slot {
			return 1
		}
		if a.Slot > b.Slot {
			return -1
		}
		return 0
	})

	cutoffSlot := response[0].Slot - lookbackDistance
	var descResults []SolanaPriorityFeeResponse
	for _, result := range response {
		if result.Slot >= cutoffSlot {
			descResults = append(descResults, SolanaPriorityFeeResponse{Slot: result.Slot, PrioritizationFee: result.PrioritizationFee})
		}
	}
	return descResults
}
