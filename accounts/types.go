package accounts

import (
	"driftgo/lib/drift"
	"driftgo/lib/event"
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	oracles "driftgo/oracles/types"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type AccountUpdateEvent[T any] struct {
	PubKey solana.PublicKey
	Data   *T
}
type IAccountSubscriber[T any] interface {
	Subscribe(onChange func(*T))
	Fetch()
	Unsubscribe()
	SetData(*T, ...uint64)
	GetDataAndSlot() *DataAndSlot[*T]
	Subscribed() bool
}

type MultiAccountSubscriber[T any] struct {
	IMultiAccountSubscriber[T]
	DataAndSlot *DataAndSlot[T]
}

type IMultiAccountSubscriber[T any] interface {
	Add(solana.PublicKey) bool
	Subscribe(func(string, *T))
	Fetch()
	Unsubscribe()
	SetData(solana.PublicKey, *T, ...uint64)
	GetDataAndSlots() map[string]*DataAndSlot[*T]
	GetDataAndSlot(string) *DataAndSlot[*T]
	Subscribed() bool
}

type AccountSubscriber[T any] struct {
	IAccountSubscriber[T]
	DataAndSlot *DataAndSlot[T]
}

type ProgramAccountSubscriberOptions struct {
	Filters    []rpc.RPCFilter
	Commitment *rpc.CommitmentType
}

type IProgramAccountSubscriber[T any] interface {
	Subscribe(onChange func(solana.PublicKey, *T, rpc.Context, []byte, ...*solana_geyser_grpc.SubscribeUpdateAccountInfo))
	Unsubscribe()
	Subscribed() bool
}

type ProgramAccountSubscriber[T any] struct {
	IProgramAccountSubscriber[T]
}

type IDriftClientAccountEvents interface {
	StateAccountUpdate(*drift.State)
	PerpMarketAccountUpdate(*drift.PerpMarket)
	SpotMarketAccountUpdate(*drift.SpotMarket)
	OraclePriceUpdate(solana.PublicKey, *oracles.OraclePriceData)
	UserAccountUpdate(*drift.User)
	Update()
	Error(error)
}

type IDriftClientMetricsEvents interface {
	TxSigned()
}

type IDriftClientAccountSubscriber interface {
	Subscribe() bool
	Fetch()
	Unsubscribe()
	Subscribed() bool

	AddPerpMarket(uint16) bool
	AddSpotMarket(uint16) bool
	AddOracle(info oracles.OracleInfo) bool
	SetPerpOracleMap()
	SetSpotOracleMap()

	GetStateAccountAndSlot() *DataAndSlot[*drift.State]
	GetMarketAccountAndSlot(uint16) *DataAndSlot[*drift.PerpMarket]
	GetMarketAccountsAndSlots() []*DataAndSlot[*drift.PerpMarket]

	GetSpotMarketAccountAndSlot(uint16) *DataAndSlot[*drift.SpotMarket]
	GetSpotMarketAccountsAndSlots() []*DataAndSlot[*drift.SpotMarket]
	GetOraclePriceDataAndSlot(solana.PublicKey) *DataAndSlot[*oracles.OraclePriceData]
	GetOraclePriceDataAndSlotForPerpMarket(uint16) *DataAndSlot[*oracles.OraclePriceData]
	GetOraclePriceDataAndSlotForSpotMarket(uint16) *DataAndSlot[*oracles.OraclePriceData]
}

type DriftClientAccountSubscriber struct {
	IDriftClientAccountSubscriber
	EventEmitter                        *event.EventEmitter
	IsSubscribed                        bool
	UpdateAccountLoaderPollingFrequency func(pollingFrequency int64)
}

type IUserAccountEvents interface {
	UserAccountUpdate(user *drift.User)
	Update()
	Error(error)
}

type IUserAccountSubscriber interface {
	Subscribe(*drift.User) bool
	Fetch()
	UpdateData(*drift.User, uint64)
	Unsubscribe()
	Subscribed() bool

	GetUserAccountAndSlot() *DataAndSlot[*drift.User]
}

type UserAccountSubscriber struct {
	IUserAccountSubscriber
	EventEmitter *event.EventEmitter
	IsSubscribed bool
}

type IOracleEvents interface {
	OracleUpdate(*oracles.OraclePriceData)
	Update()
	Error(error)
}

type IOracleAccountSubscriber interface {
	Subscribe() bool
	Fetch()
	Unsubscribe()
	Subscribed() bool

	GetOraclePriceData() *DataAndSlot[*oracles.OraclePriceData]
}

type OracleAccountSubscriber struct {
	IOracleAccountSubscriber
	EventEmitter *event.EventEmitter
	IsSubscribed bool
}

type OraclePriceUpdateEventData struct {
	Pubkey          solana.PublicKey
	OraclePriceData *oracles.OraclePriceData
}
type AccountToPoll struct {
	Key        string
	PublicKey  solana.PublicKey
	EventType  string
	CallbackId string
	MapKey     uint64
}

type OraclesToPoll struct {
	PublicKey  solana.PublicKey
	Source     drift.OracleSource
	CallbackId string
}
type Buffer []byte

type BufferAndSlot struct {
	Buffer Buffer
	Slot   uint64
}

type DataAndSlot[T any] struct {
	Data   T
	Slot   uint64
	Pubkey solana.PublicKey
}

type IUserStatsAccountEvents interface {
	UserStatsAccountUpdate(*drift.UserStats)
	Update()
	Error(error)
}

type IUserStatsAccountSubscriber interface {
	Subscribe(*drift.UserStats) bool
	Fetch()
	Unsubscribe()
	Subscribed() bool

	GetUserStatsAccountAndSlot() *DataAndSlot[*drift.UserStats]
}

type UserStatsAccountSubscriber struct {
	IUserStatsAccountSubscriber
	EventEmitter *event.EventEmitter
	IsSubscribed bool
}
