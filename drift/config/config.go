package config

import (
	"driftgo/constants"
)

type DriftEnv string

const (
	DriftEnvNone        DriftEnv = ""
	DriftEnvDevnet      DriftEnv = "devnet"
	DriftEnvMainnetBeta DriftEnv = "mainnet-beta"
)

type DriftConfig struct {
	ENV                          DriftEnv
	PYTH_ORACLE_MAPPING_ADDRESS  string
	DRIFT_PROGRAM_ID             string
	JIT_PROXY_PROGRAM_ID         string
	USDC_MINT_ADDRESS            string
	SERUM_V3                     string
	PHOENIX                      string
	V2_ALPHA_TICKET_MINT_ADDRESS string
	PERP_MARKETS                 []constants.PerpMarketConfig
	SPOT_MARKETS                 []constants.SpotMarketConfig
	MARKET_LOOKUP_TABLE          string
	SERUM_LOOKUP_TABLE           string
}

const DRIFT_PROGRAM_ID = "dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH"

var DriftConfigs = map[DriftEnv]DriftConfig{
	DriftEnvDevnet: {
		ENV:                          "devnet",
		PYTH_ORACLE_MAPPING_ADDRESS:  "BmA9Z6FjioHJPpjT39QazZyhDRUdZy2ezwx4GiDdE2u2",
		DRIFT_PROGRAM_ID:             DRIFT_PROGRAM_ID,
		JIT_PROXY_PROGRAM_ID:         "J1TnP8zvVxbtF5KFp5xRmWuvG9McnhzmBd9XGfCyuxFP",
		USDC_MINT_ADDRESS:            "8zGuJQqwhZafTah7Uc7Z4tXRnguqkn5KLFAP8oV6PHe2",
		SERUM_V3:                     "DESVgJVGajEgKGXhb6XmqDHGz3VjdgP7rEVESBgxmroY",
		PHOENIX:                      "PhoeNiXZ8ByJGLkxNfZRnkUfjvmuYqLR89jjFHGqdXY",
		V2_ALPHA_TICKET_MINT_ADDRESS: "DeEiGWfCMP9psnLGkxGrBBMEAW5Jv8bBGMN8DCtFRCyB",
		PERP_MARKETS:                 constants.DevnetPerpMarkets,
		SPOT_MARKETS:                 constants.DevnetSpotMarkets,
		MARKET_LOOKUP_TABLE:          "FaMS3U4uBojvGn5FSDEPimddcXsCfwkKsFgMVVnDdxGb",
	},
	DriftEnvMainnetBeta: {
		ENV:                          "mainnet-beta",
		PYTH_ORACLE_MAPPING_ADDRESS:  "AHtgzX45WTKfkPG53L6WYhGEXwQkN1BVknET3sVsLL8J",
		DRIFT_PROGRAM_ID:             DRIFT_PROGRAM_ID,
		JIT_PROXY_PROGRAM_ID:         "J1TnP8zvVxbtF5KFp5xRmWuvG9McnhzmBd9XGfCyuxFP",
		USDC_MINT_ADDRESS:            "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		SERUM_V3:                     "srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX",
		PHOENIX:                      "PhoeNiXZ8ByJGLkxNfZRnkUfjvmuYqLR89jjFHGqdXY",
		V2_ALPHA_TICKET_MINT_ADDRESS: "Cmvhycb6LQvvzaShGw4iDHRLzeSSryioAsU98DSSkMNa",
		PERP_MARKETS:                 constants.MainnetPerpMarkets,
		SPOT_MARKETS:                 constants.MainnetSpotMarkets,
		MARKET_LOOKUP_TABLE:          "D9cnvzswDikQDf53k4HpQ3KJ9y1Fv3HGGDFYMXnK5T6c",
		SERUM_LOOKUP_TABLE:           "GPZkp76cJtNL2mphCvT6FXkJCVPpouidnacckR6rzKDN",
	},
}
