package utils

import (
	"context"
	go_drift "driftgo"
	types2 "driftgo/anchor/types"
	"driftgo/constants"
	"driftgo/drift/config"
	"driftgo/lib/drift"
	oracles "driftgo/oracles/types"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go/rpc"
)

func GetMarketsAndOraclesForSubscription(env config.DriftEnv) (
	[]uint16,
	[]uint16,
	[]oracles.OracleInfo,
) {
	var perpMarketIndexes []uint16
	var spotMarketIndexes []uint16
	var oracleInfosMap map[string]oracles.OracleInfo = make(map[string]oracles.OracleInfo)

	for _, market := range constants.PerpMarkets[string(env)] {
		perpMarketIndexes = append(perpMarketIndexes, market.MarketIndex)
		oracleInfosMap[market.Oracle.String()] = oracles.OracleInfo{
			PublicKey: market.Oracle,
			Source:    market.OracleSource,
		}
	}
	for _, market := range constants.SpotMarkets[string(env)] {
		spotMarketIndexes = append(spotMarketIndexes, market.MarketIndex)
		oracleInfosMap[market.Oracle.String()] = oracles.OracleInfo{
			PublicKey: market.Oracle,
			Source:    market.OracleSource,
		}
	}

	return perpMarketIndexes, spotMarketIndexes, utils.MapValues(oracleInfosMap)
}

func FindAllMarketAndOracles(program types2.IProgram) (
	[]uint16,
	[]uint16,
	[]oracles.OracleInfo,
) {
	var perpMarketIndexes []uint16
	var spotMarketIndexes []uint16
	var oracleInfosMap = make(map[string]oracles.OracleInfo)
	var perpMarkets []*drift.PerpMarket
	var spotMarkets []*drift.SpotMarket

	conn := go_drift.Connec
	perpMarketAccounts, err := conn.GetProgramAccountsWithOpts(
		context.TODO(),
		program.GetProgramId(),
		&rpc.GetProgramAccountsOpts{
			Filters: []rpc.RPCFilter{
				go_drift.GetPerpMarketFilter(),
			},
		},
	)
	//perpMarketAccounts, err := program.GetProvider().GetConnection().GetProgramAccountsWithOpts(
	//	context.TODO(),
	//	program.GetProgramId(),
	//	&rpc.GetProgramAccountsOpts{
	//		Filters: []rpc.RPCFilter{
	//			go_drift.GetPerpMarketFilter(),
	//		},
	//	},
	//)
	if err != nil {
		panic("Can not load perpMarket accounts")
	}
	for _, account := range perpMarketAccounts {
		perpMarket := program.GetAccounts(
			drift.PerpMarket{},
			"PerpMarket",
		).Decode(
			account.Account.Data.GetBinary(),
		).(*drift.PerpMarket)

		if perpMarket == nil {
			continue
		}
		perpMarkets = append(perpMarkets, perpMarket)
	}

	spotMarketAccounts, err := conn.GetProgramAccountsWithOpts(
		context.TODO(),
		program.GetProgramId(),
		&rpc.GetProgramAccountsOpts{
			Filters: []rpc.RPCFilter{
				go_drift.GetSpotMarketFilter(),
			},
		},
	)

	//spotMarketAccounts, err := program.GetProvider().GetConnection().GetProgramAccountsWithOpts(
	//	context.TODO(),
	//	program.GetProgramId(),
	//	&rpc.GetProgramAccountsOpts{
	//		Filters: []rpc.RPCFilter{
	//			go_drift.GetSpotMarketFilter(),
	//		},
	//	},
	//)
	if err != nil {
		panic("Can not load spotMarket accounts")
	}
	for _, account := range spotMarketAccounts {
		spotMarket := program.GetAccounts(
			drift.SpotMarket{},
			"SpotMarket",
		).Decode(
			account.Account.Data.GetBinary(),
		).(*drift.SpotMarket)
		if spotMarket == nil {
			continue
		}
		spotMarkets = append(spotMarkets, spotMarket)
	}
	for _, perpMarket := range perpMarkets {
		perpMarketIndexes = append(perpMarketIndexes, perpMarket.MarketIndex)
		oracleInfosMap[perpMarket.Amm.Oracle.String()] = oracles.OracleInfo{
			PublicKey: perpMarket.Amm.Oracle,
			Source:    perpMarket.Amm.OracleSource,
		}
	}

	for _, spotMarket := range spotMarkets {
		spotMarketIndexes = append(spotMarketIndexes, spotMarket.MarketIndex)
		oracleInfosMap[spotMarket.Oracle.String()] = oracles.OracleInfo{
			PublicKey: spotMarket.Oracle,
			Source:    spotMarket.OracleSource,
		}
	}

	return perpMarketIndexes, spotMarketIndexes, utils.MapValues(oracleInfosMap)
}
