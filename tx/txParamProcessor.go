package tx

import (
	"context"
	go_drift "driftgo"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	"math"
)

const COMPUTE_UNIT_BUFFER_FACTOR = 1.2

const TEST_SIMS_ALWAYS_FAIL = false

type TransactionProps struct {
	Instructions []solana.Instruction
	TxParams     go_drift.BaseTxParams
	LookupTables []addresslookuptable.KeyedAddressLookupTable
}

func GetComputeUnitsFromSim(
	txSim *rpc.SimulateTransactionResponse,
) *uint64 {
	if txSim != nil && txSim.Value != nil && txSim.Value.UnitsConsumed != nil {
		return &(*txSim.Value.UnitsConsumed)
	}
	return nil
}

func GetTxSimComputeUnits(
	tx *solana.Transaction,
	connection *rpc.Client,
) (bool, uint64) {
	if TEST_SIMS_ALWAYS_FAIL {
		panic("Test Error::SIMS_ALWAYS_FAIL")
	}
	simTxResult, err := connection.SimulateTransactionWithOpts(
		context.TODO(),
		tx,
		&rpc.SimulateTransactionOpts{
			ReplaceRecentBlockhash: true,
		},
	)
	if err != nil {
		return false, 0
	}

	if simTxResult != nil && simTxResult.Value != nil && simTxResult.Value.Err != nil {
		panic(simTxResult.Value.Err)
	}

	computeUnits := GetComputeUnitsFromSim(simTxResult)
	if computeUnits == nil {
		return false, 0
	}
	return true, *computeUnits
}

func ProcessTxParams(
	txProps *TransactionProps,
	txBuilder func(*TransactionProps) *solana.Transaction,
	processConfig *go_drift.ProcessingTxParams,
	connection *rpc.Client,
) go_drift.BaseTxParams {
	// # Exit early if no process config is provided
	if processConfig == nil ||
		(processConfig.UseSimulatedComputeUnits == nil &&
			processConfig.UseSimulateComputeUnitsForCUPriceCalculation == nil &&
			processConfig.GetCUPriceFromComputeUnits == nil &&
			processConfig.ComputeUnitsBufferMultiplier == nil) {
		return txProps.TxParams
	}

	// # Setup
	finalTxProps := *txProps

	// # Run Process
	if *processConfig.UseSimulatedComputeUnits {
		txParams := txProps.TxParams
		txParams.ComputeUnits = 1_400_000
		txToSim := txBuilder(&TransactionProps{
			Instructions: txProps.Instructions,
			TxParams:     txParams,
			LookupTables: txProps.LookupTables,
		})
		sucess, computeUnits := GetTxSimComputeUnits(txToSim, connection)
		if sucess {
			multiplier := utils.TTM[float64](
				processConfig.ComputeUnitsBufferMultiplier != nil,
				func() float64 { return *processConfig.ComputeUnitsBufferMultiplier },
				COMPUTE_UNIT_BUFFER_FACTOR,
			)
			bufferedComputeUnits := float64(computeUnits) * multiplier

			// Adjust the transaction based on the simulated compute units
			finalTxProps.TxParams.ComputeUnits = uint64(math.Ceil(bufferedComputeUnits))
		}
	}
	if processConfig.UseSimulateComputeUnitsForCUPriceCalculation != nil && *processConfig.UseSimulateComputeUnitsForCUPriceCalculation {
		if processConfig.UseSimulatedComputeUnits == nil || !*processConfig.UseSimulatedComputeUnits {
			panic("encountered useSimulatedComputeUnitsForFees=true, but useSimulatedComputeUnits is false")
		}
		if processConfig.GetCUPriceFromComputeUnits == nil {
			panic("encountered useSimulatedComputeUnitsForFees=true, but getComputeUnitPriceFromUnitsToUse helper method is undefined")
		}

		simulatedComputeUnits := finalTxProps.TxParams.ComputeUnits

		finalTxProps.TxParams.ComputeUnitsPrice = processConfig.GetCUPriceFromComputeUnits(simulatedComputeUnits)
	}
	// # Return Final Tx Params
	return finalTxProps.TxParams
}
