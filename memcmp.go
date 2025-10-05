package go_drift

import (
	"crypto/sha256"
	"fmt"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/iancoleman/strcase"
)

const DISCRIMINATOR_SIZE = 8

func GetAccountFilter(accountName string) rpc.RPCFilter {
	hash := sha256.Sum256([]byte(fmt.Sprintf("account:%s", strcase.ToCamel(accountName))))
	hashCut := hash[0:DISCRIMINATOR_SIZE]
	return rpc.RPCFilter{

		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 0,
			Bytes:  hashCut[:],
		},
	}
}
func GetUserFilter() rpc.RPCFilter {
	return GetAccountFilter("User")
}

func GetNonIdleUserFiler() rpc.RPCFilter {
	return rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 4350,
			Bytes:  []byte{0},
		},
	}
}

func GetPerpMarketFilter() rpc.RPCFilter {
	return GetAccountFilter("PerpMarket")
}

func GetSpotMarketFilter() rpc.RPCFilter {
	return GetAccountFilter("SpotMarket")
}

func GetUserWithOrderFilter() rpc.RPCFilter {
	return rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 4352,
			Bytes:  []byte{1},
		},
	}
}

func GetUserWithAuctionFilter() rpc.RPCFilter {
	return rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 4354,
			Bytes:  []byte{1},
		},
	}
}

func GetUserThatHasBeenLP() rpc.RPCFilter {
	return rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 4267,
			Bytes:  []byte{99},
		},
	}
}

func GetUserWithName() rpc.RPCFilter {
	return rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 72,
			Bytes:  []byte{99},
		},
	}
}
