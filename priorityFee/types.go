package priorityFee

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const DEFAULT_PRIORITY_FEE_MAP_FREQUENCY_MS = int64(10_000)

type IPriorityFeeStrategy interface {
	Calculate(samples []SolanaPriorityFeeResponse) uint64
}

type PriorityFeeMethod string

const (
	PriorityFeeMethodSolana = "solana"
	PriorityFeeMethodHelius = "helius"
	PriorityFeeMethodDrift  = "drift"
)

type PriorityFeeSubscriberConfig struct {
	/// rpc connection, optional if using priorityFeeMethod.HELIUS
	Connection *rpc.Client
	/// frequency to make RPC calls to update priority fee samples, in milliseconds
	FrequencyMs uint64
	/// addresses you plan to write lock, used to determine priority fees
	Addresses []solana.PublicKey
	/// drift market type and index, optionally provide at initialization time if using priorityFeeMethod.DRIFT
	DriftMarkets []DriftMarketInfo
	/// custom strategy to calculate priority fees, defaults to AVERAGE
	CustomStrategy IPriorityFeeStrategy
	/// method for fetching priority fee samples
	PriorityFeeMethod PriorityFeeMethod
	/// lookback window to determine priority fees, in slots.
	SlotsToCheck uint64
	/// url for helius rpc, required if using priorityFeeMethod.HELIUS
	HeliusRpcUrl string
	/// url for drift cached priority fee endpoint, required if using priorityFeeMethod.DRIFT
	DriftPriorityFeeEndpoint string
	/// clamp any returned priority fee value to this value.
	MaxFeeMicroLamports uint64
	/// multiplier applied to priority fee before maxFeeMicroLamports, defaults to 1.0
	PriorityFeeMultiplier float64

	Percentile uint

	Callback func(*PriorityFeeSubscriber)
}

type PriorityFeeSubscriberMapConfig struct {
	/// rpc connection, optional if using priorityFeeMethod.HELIUS
	Connection *rpc.Client
	/// frequency to make RPC calls to update priority fee samples, in milliseconds
	FrequencyMs *int64
	/// addresses you plan to write lock, used to determine priority fees
	Addresses []solana.PublicKey
	/// drift market type and index, optionally provide at initialization time if using priorityFeeMethod.DRIFT
	DriftMarkets []DriftMarketInfo
	/// custom strategy to calculate priority fees, defaults to AVERAGE
	CustomStrategy IPriorityFeeStrategy
	/// method for fetching priority fee samples
	PriorityFeeMethod *PriorityFeeMethod
	/// lookback window to determine priority fees, in slots.
	SlotsToCheck *uint64
	/// url for helius rpc, required if using priorityFeeMethod.HELIUS
	HeliusRpcUrl string
	/// url for drift cached priority fee endpoint, required if using priorityFeeMethod.DRIFT
	DriftPriorityFeeEndpoint string
	/// clamp any returned priority fee value to this value.
	MaxFeeMicroLamports *uint32
	/// multiplier applied to priority fee before maxFeeMicroLamports, defaults to 1.0
	PriorityFeeMultiplier *float64
}
