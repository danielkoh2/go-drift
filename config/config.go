package config

import (
	"driftgo/drift/config"
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
