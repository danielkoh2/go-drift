package priorityFee

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// GetRecentPrioritizationFees returns a list of prioritization fees from recent blocks.
// Currently, a node's prioritization-fee cache stores data from up to 150 blocks.
func GetRecentPrioritizationFeesEx(
	cl *rpc.Client,
	ctx context.Context,
	accounts solana.PublicKeySlice, // optional
	percentile uint,
) (out []rpc.PriorizationFeeResult, err error) {
	params := []interface{}{accounts}
	if percentile > 0 {
		obj := rpc.M{}
		obj["percentile"] = percentile
		params = append(params, obj)
	}

	err = cl.RPCCallForInto(ctx, &out, "getRecentPrioritizationFees", params)
	return
}
