package types

import (
	"driftgo/anchor/types"
	"driftgo/lib/drift"
	"driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go/rpc"
)

func GetOracleClient(
	oracleSource drift.OracleSource,
	connection *rpc.Client,
	program types.IProgram,
) IOracleClient {
	if oracleSource == drift.OracleSource_Pyth {
		return CreatePythClient(connection, utils.BN(1), false)
	}

	//if oracleSource == drift.OracleSource_PythPull{return CreatePythPullClient(connection, utils.BN(1), false)}

	if oracleSource == drift.OracleSource_Pyth1K {
		return CreatePythClient(connection, utils.BN(1000), false)
	}

	if oracleSource == drift.OracleSource_Pyth1M {
		return CreatePythClient(connection, utils.BN(1000000), false)
	}

	if oracleSource == drift.OracleSource_PythStableCoin {
		return CreatePythClient(connection, utils.BN(1), true)
	}

	if oracleSource == drift.OracleSource_Switchboard {
		return CreateSwitchboardClient(connection)
	}

	if oracleSource == drift.OracleSource_Prelaunch {
		return CreatePrelaunchOracleClient(connection, program)
	}

	if oracleSource == drift.OracleSource_QuoteAsset {
		return CreateQuoteAssetOracleClient()
	}

	if oracleSource == drift.OracleSource_PythLazer {
		return CreatePythClient(connection, utils.BN(1), false)
	}
	panic(fmt.Sprintf("Unknown oracle source %d", oracleSource))
}
