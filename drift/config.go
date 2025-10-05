package drift

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

var CurrentConfig = config.DriftConfigs[config.DriftEnvDevnet]

func GetConfig() *config.DriftConfig {
	return &CurrentConfig
}

func Initialize(env config.DriftEnv, overrideConfig *config.DriftConfig) *config.DriftConfig {
	CurrentConfig = config.DriftConfigs[env]
	if overrideConfig != nil {
		if overrideConfig.PYTH_ORACLE_MAPPING_ADDRESS != "" {
			CurrentConfig.PYTH_ORACLE_MAPPING_ADDRESS = overrideConfig.PYTH_ORACLE_MAPPING_ADDRESS
		}
		if overrideConfig.DRIFT_PROGRAM_ID != "" {
			CurrentConfig.DRIFT_PROGRAM_ID = overrideConfig.DRIFT_PROGRAM_ID
		}
		if overrideConfig.JIT_PROXY_PROGRAM_ID != "" {
			CurrentConfig.JIT_PROXY_PROGRAM_ID = overrideConfig.JIT_PROXY_PROGRAM_ID
		}
		if overrideConfig.USDC_MINT_ADDRESS != "" {
			CurrentConfig.USDC_MINT_ADDRESS = overrideConfig.USDC_MINT_ADDRESS
		}
		if overrideConfig.SERUM_V3 != "" {
			CurrentConfig.SERUM_V3 = overrideConfig.SERUM_V3
		}
		if overrideConfig.PHOENIX != "" {
			CurrentConfig.PHOENIX = overrideConfig.PHOENIX
		}
		if overrideConfig.V2_ALPHA_TICKET_MINT_ADDRESS != "" {
			CurrentConfig.V2_ALPHA_TICKET_MINT_ADDRESS = overrideConfig.V2_ALPHA_TICKET_MINT_ADDRESS
		}
		if overrideConfig.MARKET_LOOKUP_TABLE != "" {
			CurrentConfig.MARKET_LOOKUP_TABLE = overrideConfig.MARKET_LOOKUP_TABLE
		}
		if overrideConfig.SERUM_LOOKUP_TABLE != "" {
			CurrentConfig.SERUM_LOOKUP_TABLE = overrideConfig.SERUM_LOOKUP_TABLE
		}
	}
	return &CurrentConfig
}

func GetMarketsAndOraclesForSubscription(env config.DriftEnv) (
	[]uint16,
	[]uint16,
	[]oracles.OracleInfo,
) {
	var perpMarketIndexes []uint16
	var spotMarketIndexes []uint16
	oracleInfosMap := make(map[string]oracles.OracleInfo)

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

	return perpMarketIndexes, spotMarketIndexes, utils.MapValues[string, oracles.OracleInfo](oracleInfosMap)
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

	perpMarketAccounts, err := program.GetProvider().GetConnection().GetProgramAccountsWithOpts(
		context.TODO(),
		program.GetProgramId(),
		&rpc.GetProgramAccountsOpts{
			Filters: []rpc.RPCFilter{
				go_drift.GetPerpMarketFilter(),
			},
		},
	)
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

	spotMarketAccounts, err := program.GetProvider().GetConnection().GetProgramAccountsWithOpts(
		context.TODO(),
		program.GetProgramId(),
		&rpc.GetProgramAccountsOpts{
			Filters: []rpc.RPCFilter{
				go_drift.GetSpotMarketFilter(),
			},
		},
	)
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

	return perpMarketIndexes, spotMarketIndexes, utils.MapValues[string, oracles.OracleInfo](oracleInfosMap)
}
