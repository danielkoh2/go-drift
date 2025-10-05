package phoenix

import (
	"context"
	"driftgo/lib/phoenix/types"
	errors2 "errors"
	"fmt"
	agBinary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-errors/errors"
	"math"
	"slices"
)

type OrderId struct {
	PriceInTicks        uint64
	OrderSequenceNumber uint64
}

type RestingOrder struct {
	TraderIndex                     uint64
	NumBaseLots                     uint64
	LastValidSlot                   uint64
	LastValidUnixTimestampInSeconds int64
}

type TraderState struct {
	QuoteLotsLocked uint64
	QuoteLotsFee    uint64
	BaseLotsLocked  uint64
	BaseLotsFee     uint64
	Padding         [8]uint64
}

type OrderBook struct {
	OrderId      OrderId
	RestingOrder RestingOrder
}

type LadderLevel struct {
	PriceInTicks   uint64
	SizeInBaseLots uint64
}

type Ladder struct {
	Bids []LadderLevel
	Asks []LadderLevel
}

type L3Order struct {
	PriceInTicks                    uint64
	Side                            types.Side
	SizeInBaseLots                  uint64
	MakerPubkey                     string
	OrderSequenceNumber             uint64
	LastValidSlot                   uint64
	LastValidUnixTimestampInSeconds int64
}

type L3UiOrder struct {
	Price                           uint64
	Side                            types.Side
	Size                            uint64
	MakerPubkey                     string
	OrderSequenceNumber             uint64
	LastValidSlot                   uint64
	LastValidUnixTimestampInSeconds int64
}

type L3Book struct {
	Bids []L3Order
	Asks []L3Order
}

type L3UiBook struct {
	Bids []L3Order
	Asks []L3Order
}

type UiLadderLevel struct {
	Price    uint64
	Quantity uint64
}

type UiLadder struct {
	Bids []UiLadderLevel
	Asks []UiLadderLevel
}

func (obj OrderId) UnmarshalWithDecoder(decoder *agBinary.Decoder) error {
	err := decoder.Decode(&obj.PriceInTicks)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.OrderSequenceNumber)
	if err != nil {
		return err
	}
	return nil
}

func (obj RestingOrder) UnmarshalWithDecoder(decoder *agBinary.Decoder) error {
	err := decoder.Decode(&obj.TraderIndex)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.NumBaseLots)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.LastValidSlot)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.LastValidUnixTimestampInSeconds)
	if err != nil {
		return err
	}
	return nil
}

func (obj TraderState) UnmarshalWithDecoder(decoder *agBinary.Decoder) error {
	err := decoder.Decode(&obj.QuoteLotsLocked)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.QuoteLotsFee)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.BaseLotsLocked)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.BaseLotsFee)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.Padding)
	if err != nil {
		return err
	}
	return nil
}

func (obj OrderBook) UnmarshalWithDecoder(decoder *agBinary.Decoder) error {
	err := decoder.Decode(&obj.OrderId)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.RestingOrder)
	if err != nil {
		return err
	}
	return nil
}

type MarketData struct {
	// The raw MarketHeader from the market account
	Header MarketHeader

	// The number of base lots per base unit
	BaseLotsPerBaseUnit uint64

	// Tick size of the market, in quote lots per base unit
	// Note that the header contains tick size in quote atoms per base unit
	QuoteLotsPerBaseUnitPerTick uint64

	// The next order sequence number of the market
	SequenceNumber uint64

	// Taker fee in basis points
	TakerFeeBps uint64

	// Total fees collected by the market and claimed by fee recipient, in quote lots
	CollectedQuoteLotFees uint64

	// Total unclaimed fees in the market, in quote lots
	UnclaimedQuoteLotFees uint64

	// The bids on the market, sorted from highest to lowest price
	Bids []OrderBook

	// The asks on the market, sorted from lowest to highest price
	Asks []OrderBook

	// Map from trader pubkey to trader state
	Traders map[string]TraderState

	// Map from trader pubkey to trader index
	TraderPubkeyToTraderIndex map[string]int

	// Map from trader index to trader pubkey
	TraderIndexToTraderPubkey map[int]string
}

type Market struct {
	Address solana.PublicKey
	Data    *MarketData

	// Optional fields containing the name of the market and the tokens used for the market
	Name       string
	BaseToken  *Token
	QuoteToken *Token
}

func (p *Market) Reload(buffer []byte) *Market {
	decoder := agBinary.NewBinDecoder(buffer)
	var marketData MarketData
	err := decoder.Decode(&marketData)
	if err != nil {
		fmt.Println("Phoenix market reload failed: err=", err)
		return p
	}
	p.Data = &marketData

	return p
}

func (p *Market) GetUiLadder(
	levels int,
	slot uint64,
	unixTimestamp int64,
) *UiLadder {
	ladder := p.GetLadder(slot, unixTimestamp, levels)

	var bids []UiLadderLevel
	var asks []UiLadderLevel
	for _, bid := range ladder.Bids {
		bids = append(bids, p.LevelToUiLevel(bid.PriceInTicks, bid.SizeInBaseLots))
	}
	for _, ask := range ladder.Asks {
		asks = append(asks, p.LevelToUiLevel(ask.PriceInTicks, ask.SizeInBaseLots))
	}
	return &UiLadder{
		Bids: bids,
		Asks: asks,
	}
}

func (p *Market) GetLadder(
	slot uint64,
	unixTimestamp int64,
	levels int,
) *Ladder {
	var bids []LadderLevel
	var asks []LadderLevel

	for _, bid := range p.Data.Bids {
		if bid.RestingOrder.LastValidSlot != 0 && bid.RestingOrder.LastValidSlot < slot {
			continue
		}
		if bid.RestingOrder.LastValidUnixTimestampInSeconds != 0 &&
			bid.RestingOrder.LastValidUnixTimestampInSeconds < unixTimestamp {
			continue
		}
		priceInTicks := bid.OrderId.PriceInTicks
		sizeInBaseLots := bid.RestingOrder.NumBaseLots
		if len(bids) == 0 {
			bids = append(bids, LadderLevel{
				PriceInTicks:   priceInTicks,
				SizeInBaseLots: sizeInBaseLots,
			})
		} else {
			prev := bids[len(bids)-1]
			if priceInTicks == prev.PriceInTicks {
				prev.SizeInBaseLots += sizeInBaseLots
			} else {
				if len(bids) == levels {
					break
				}
				bids = append(bids, LadderLevel{
					PriceInTicks:   priceInTicks,
					SizeInBaseLots: sizeInBaseLots,
				})
			}
		}
	}
	for _, ask := range p.Data.Asks {
		if ask.RestingOrder.LastValidSlot != 0 && ask.RestingOrder.LastValidSlot < slot {
			continue
		}
		if ask.RestingOrder.LastValidUnixTimestampInSeconds != 0 &&
			ask.RestingOrder.LastValidUnixTimestampInSeconds < unixTimestamp {
			continue
		}
		priceInTicks := ask.OrderId.PriceInTicks
		sizeInBaseLots := ask.RestingOrder.NumBaseLots
		if len(asks) == 0 {
			asks = append(asks, LadderLevel{
				PriceInTicks:   priceInTicks,
				SizeInBaseLots: sizeInBaseLots,
			})
		} else {
			prev := asks[len(asks)-1]
			if priceInTicks == prev.PriceInTicks {
				prev.SizeInBaseLots += sizeInBaseLots
			} else {
				if len(asks) == levels {
					break
				}
				asks = append(asks, LadderLevel{
					PriceInTicks:   priceInTicks,
					SizeInBaseLots: sizeInBaseLots,
				})
			}
		}
	}
	return &Ladder{
		Bids: bids,
		Asks: asks,
	}
}

func (p *Market) LevelToUiLevel(
	priceInTicks uint64,
	sizeInBaseLots uint64,
) UiLadderLevel {
	return UiLadderLevel{
		Price:    p.TicksToFloatPrice(priceInTicks),
		Quantity: p.BaseLotsToRawBaseUnits(sizeInBaseLots),
	}
}

func (p *Market) TicksToFloatPrice(ticks uint64) uint64 {
	return (ticks * p.Data.QuoteLotsPerBaseUnitPerTick * p.Data.Header.QuoteLotSize) /
		(uint64(math.Pow(10, float64(p.Data.Header.QuoteParams.Decimals))) * uint64(p.Data.Header.RawBaseUnitsPerBaseUnit))
}

func (p *Market) RawBaseUnitsToBaseLotsRoundedDown(rawBaseUnits uint64) uint64 {
	baseUnits := rawBaseUnits / uint64(p.Data.Header.RawBaseUnitsPerBaseUnit)
	return uint64(math.Floor(float64(baseUnits * p.Data.BaseLotsPerBaseUnit)))
}

func (p *Market) BaseLotsToBaseAtoms(baseLots uint64) uint64 {
	return baseLots * p.Data.Header.BaseLotSize
}

func (p *Market) BaseAtomsToRawBaseUnits(baseAtoms uint64) uint64 {
	return baseAtoms / uint64(math.Pow(10, float64(p.Data.Header.BaseParams.Decimals)))
}

func (p *Market) BaseLotsToRawBaseUnits(baseLots uint64) uint64 {
	return p.BaseAtomsToRawBaseUnits(p.BaseLotsToBaseAtoms(baseLots))
}
func (obj MarketData) UnmarshalWithDecoder(decoder *agBinary.Decoder) error {
	err := decoder.Decode(&obj.Header)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.BaseLotsPerBaseUnit)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.QuoteLotsPerBaseUnitPerTick)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.SequenceNumber)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.TakerFeeBps)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.CollectedQuoteLotFees)
	if err != nil {
		return err
	}
	err = decoder.Decode(&obj.UnclaimedQuoteLotFees)
	if err != nil {
		return err
	}

	numBids := int(obj.Header.MarketSizeParams.BidsSize)
	numAsks := int(obj.Header.MarketSizeParams.AsksSize)
	numTraders := int(obj.Header.MarketSizeParams.NumSeats)

	bidsSize := 16 + 16 + (16+16+32)*numBids
	asksSize := 16 + 16 + (16+16+32)*numAsks
	tradersSize := 16 + 16 + (16+32+96)*numTraders

	bidBuffer, _ := decoder.ReadNBytes(bidsSize)
	askBuffer, _ := decoder.ReadNBytes(asksSize)
	traderBuffer, _ := decoder.ReadNBytes(tradersSize)

	bids, err1 := deserializeRedBlackTree[OrderId, RestingOrder](bidBuffer)
	if err1 != nil {
		return err1
	}
	asks, err1 := deserializeRedBlackTree[OrderId, RestingOrder](askBuffer)
	if err1 != nil {
		return err1
	}

	// Sort bids in descending order of price, and ascending order of sequence number
	slices.SortFunc(bids, func(a, b KeyValuePair[OrderId, RestingOrder]) int {
		if a.Key.PriceInTicks < b.Key.PriceInTicks {
			return 1
		} else if a.Key.PriceInTicks > b.Key.PriceInTicks {
			return -1
		}
		aSeq := getUniOrderSequenceNumber(a.Key)
		bSeq := getUniOrderSequenceNumber(b.Key)
		if aSeq > bSeq {
			return 1
		} else if aSeq < bSeq {
			return -1
		}
		return 0
	})
	// Sort asks in descending order of price, and ascending order of sequence number
	slices.SortFunc(asks, func(a, b KeyValuePair[OrderId, RestingOrder]) int {
		if b.Key.PriceInTicks < a.Key.PriceInTicks {
			return 1
		} else if b.Key.PriceInTicks > a.Key.PriceInTicks {
			return -1
		}
		aSeq := getUniOrderSequenceNumber(a.Key)
		bSeq := getUniOrderSequenceNumber(b.Key)
		if aSeq > bSeq {
			return 1
		} else if aSeq < bSeq {
			return -1
		}
		return 0

	})
	var traders map[string]TraderState
	traderStates, err2 := deserializeRedBlackTree[solana.PublicKey, TraderState](traderBuffer)
	if err2 != nil {
		return err2
	}
	for _, keyValue := range traderStates {
		traders[keyValue.Key.String()] = keyValue.Value
	}

	var traderPubkeyToTraderIndex map[string]int
	var traderIndexToTraderPubkey map[int]string

	for k, index := range getNodeIndices[solana.PublicKey, TraderState](traderBuffer) {
		traderPubkeyToTraderIndex[k.String()] = index
		traderIndexToTraderPubkey[index] = k.String()
	}
	for _, keyValue := range bids {
		obj.Bids = append(obj.Bids, OrderBook{
			OrderId:      keyValue.Key,
			RestingOrder: keyValue.Value,
		})
	}
	for _, keyValue := range asks {
		obj.Asks = append(obj.Asks, OrderBook{
			OrderId:      keyValue.Key,
			RestingOrder: keyValue.Value,
		})
	}
	obj.Traders = traders
	obj.TraderPubkeyToTraderIndex = traderPubkeyToTraderIndex
	obj.TraderIndexToTraderPubkey = traderIndexToTraderPubkey
	return nil
}

type KeyValuePair[K OrderId | solana.PublicKey, V any] struct {
	Key   K
	Value V
}

type FreeListPointer struct {
	v1 int32
	v2 int32
}

func deserializeRedBlackTree[K OrderId | solana.PublicKey, V any](
	data []byte,
) ([]KeyValuePair[K, V], error) {
	var tree []KeyValuePair[K, V]
	nodes, freeNodes, err := deserializeRedBlackTreeNodes[K, V](data)
	if err != nil {
		return nil, err
	}
	for index, keyValue := range nodes {
		_, exists := freeNodes[index]
		if !exists {
			tree = append(tree, keyValue)
		}
	}
	return tree, nil
}

func deserializeRedBlackTreeNodes[K OrderId | solana.PublicKey, V any](
	data []byte,
) ([]KeyValuePair[K, V], map[int]bool, error) {
	decoder := agBinary.NewBinDecoder(data)
	var nodes []KeyValuePair[K, V]

	// Skip RBTree header
	err := decoder.SkipBytes(16)
	if err != nil {
		return nil, nil, err
	}
	// Skip node allocator size
	err = decoder.SkipBytes(8)
	if err != nil {
		return nil, nil, err
	}
	var bumpIndex int32
	err = decoder.Decode(&bumpIndex)
	if err != nil {
		return nil, nil, err
	}
	var freeListHead int32
	err = decoder.Decode(&freeListHead)
	if err != nil {
		return nil, nil, err
	}

	var freeListPointers []FreeListPointer

	for index := int32(0); decoder.HasRemaining() && index < bumpIndex-1; index++ {
		var registers []int32
		for i := 0; i < 4; i++ {
			var register int32
			err = decoder.Decode(&register)
			if err != nil {
				return nil, nil, err
			}
			registers = append(registers, register)
		}
		var key K
		err = decoder.Decode(&key)
		if err != nil {
			return nil, nil, err
		}
		var value V
		err = decoder.Decode(&value)
		if err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, KeyValuePair[K, V]{Key: key, Value: value})
		freeListPointers = append(freeListPointers, FreeListPointer{
			v1: index,
			v2: registers[0],
		})
	}
	var freeNodes map[int]bool
	indexToRemove := freeListHead - 1

	counter := int32(0)
	// If there's an infinite loop here, that means that the state is corrupted
	for freeListHead < bumpIndex {
		// We need to subtract 1 because the node allocator is 1-indexed
		next := freeListPointers[freeListHead-1]
		indexToRemove = next.v1
		freeListHead = next.v2
		freeNodes[int(indexToRemove)] = true
		counter += 1
		if counter > bumpIndex {
			panic(errors2.New("infinite loop detected"))
		}
	}
	return nodes, freeNodes, nil
}

func testn(num uint64, bit int) bool {
	r := uint64(bit % 26)
	s := (uint64(bit) - r) / 26
	q := uint64(1 << r)

	if 64 <= s {
		return false
	}

	w := uint64((num >> s * 26) & 0x3ffffff)

	return (w & q) != 0
}

func notn(num uint64, width int) uint64 {
	width = min(width, 64)
	r := width % 26
	s := (width - r) / 26
	var newNum uint64 = 0
	for i := 0; i < s; i++ {
		t := (^num >> i * 26) & 0x3ffffff
		newNum |= t << i * 26
	}
	if r > 0 {
		t := (^num >> s * 26) & (0x3ffffff >> (26 - r))
		newNum |= t << s * 26
	}
	return newNum
}

func ineg(num uint64) uint64 {
	return (^num & 0x8000000000000000) | num&0x7fffffffffffffff
}

func isNeg(num uint64) bool {
	return num&0x8000000000000000 != 0
}

func fromTwos(num uint64, bit int) uint64 {
	if testn(num, bit-1) {
		return ineg(notn(num, bit) + 1)
	}
	return num
}

func getNodeIndices[K OrderId | solana.PublicKey, V any](
	data []byte,
) map[K]int {
	var indexMap map[K]int
	nodes, freeNodes, _ := deserializeRedBlackTreeNodes[K, V](data)

	for index, keyValue := range nodes {
		_, exists := freeNodes[index]
		if !exists {
			indexMap[keyValue.Key] = index + 1
		}
	}

	return indexMap
}

func getUniOrderSequenceNumber(orderId OrderId) uint64 {
	seq := fromTwos(orderId.OrderSequenceNumber, 64)
	if isNeg(seq) {
		return ineg(seq) - 1
	} else {
		return seq
	}
}

func LoadMarketFromAddress(
	connection *rpc.Client,
	address solana.PublicKey,
) *Market {
	accountInfo, err := connection.GetAccountInfo(context.TODO(), address)
	if err != nil {
		return nil
	}
	var marketData MarketData
	err = agBinary.NewBinDecoder(accountInfo.GetBinary()).Decode(&marketData)
	if err != nil {
		fmt.Println("Phoenix load failed: pubkey=", address, ",err=", err, ",len=", len(accountInfo.GetBinary()))
		fmt.Println(errors.Wrap(err, 2).ErrorStack())
		return nil
	}
	return &Market{
		Address: address,
		Data:    &marketData,
	}
}

func LoadMarketFromBuffer(
	address solana.PublicKey,
	buffer []byte,
) *Market {
	var marketData MarketData
	err := agBinary.NewBinDecoder(buffer).Decode(&marketData)
	if err != nil {
		fmt.Println("Phoenix load failed: pubkey=", address, ",err=", err, ",len=", len(buffer))
		fmt.Println(errors.Wrap(err, 2).ErrorStack())
		return nil
	}
	return &Market{
		Address: address,
		Data:    &marketData,
	}
}
