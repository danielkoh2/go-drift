package solana

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type GetProgramAccountsContextResult struct {
	Context rpc.Context
	Value   []*rpc.KeyedAccount
}

func GetProgramAccountsContextWithOpts(
	cl *rpc.Client,
	ctx context.Context,
	publicKey solana.PublicKey,
	opts *rpc.GetProgramAccountsOpts,
) (out GetProgramAccountsContextResult, err error) {
	obj := rpc.M{
		"encoding":    "base64",
		"withContext": true,
	}
	if opts != nil {
		if opts.Commitment != "" {
			obj["commitment"] = string(opts.Commitment)
		}
		if len(opts.Filters) != 0 {
			obj["filters"] = opts.Filters
		}
		if opts.Encoding != "" {
			obj["encoding"] = opts.Encoding
		}
		if opts.DataSlice != nil {
			obj["dataSlice"] = rpc.M{
				"offset": opts.DataSlice.Offset,
				"length": opts.DataSlice.Length,
			}
		}
	}

	params := []interface{}{publicKey, obj}

	err = cl.RPCCallForInto(ctx, &out, "getProgramAccounts", params)
	return
}
