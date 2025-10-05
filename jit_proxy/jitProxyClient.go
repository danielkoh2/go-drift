package jit_proxy

import (
	go_drift "driftgo"
	"driftgo/constants"
	"driftgo/drift/types"
	types2 "driftgo/jit_proxy/types"
	"driftgo/lib/drift"
	"driftgo/lib/jit_proxy"
	"driftgo/tx"
	"driftgo/utils"
	"errors"
	"slices"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
)

type JitProxyClient struct {
	driftClient types.IDriftClient
}

func CreateJitProxyClient(driftClient types.IDriftClient) *JitProxyClient {
	return &JitProxyClient{
		driftClient: driftClient,
	}
}

func (p *JitProxyClient) Jit(
	params *types2.JitTxParams,
	txParams *go_drift.TxParams,
) (*tx.TxSigAndSlot, error) {
	ix, err := p.GetJitIx(params)
	if err != nil {
		return nil, err
	}
	tx := p.driftClient.BuildTransaction([]solana.Instruction{ix}, txParams, []addresslookuptable.KeyedAddressLookupTable{})
	return p.driftClient.SendTransaction(tx, nil, false)
}

func (p *JitProxyClient) GetJitIx(params *types2.JitTxParams) (solana.Instruction, error) {
	subAccountId := utils.TTM[uint16](params.SubAccountId == nil,
		func() uint16 { return p.driftClient.GetActiveSubAccountId() },
		func() uint16 { return *params.SubAccountId },
	)
	var order *drift.Order
	for idx := 0; idx < len(params.Taker.Orders); idx++ {
		if params.Taker.Orders[idx].OrderId == params.TakerOrderId {
			order = &params.Taker.Orders[idx]
			break
		}
	}
	if order == nil {
		return nil, errors.New("taker Order is not valid")
	}
	remainingAccounts := p.driftClient.GetRemainingAccounts(&types.RemainingAccountParams{
		UserAccounts:              []*drift.User{params.Taker, p.driftClient.GetUserAccount(&subAccountId, nil)},
		WritableSpotMarketIndexes: utils.TT(order.MarketType == drift.MarketType_Spot, []uint16{order.MarketIndex, constants.QUOTE_SPOT_MARKET_INDEX}, []uint16{}),
		WritablePerpMarketIndexes: utils.TT(order.MarketType == drift.MarketType_Perp, []uint16{order.MarketIndex}, []uint16{}),
	})

	if params.ReferrerInfo != nil {
		remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
			PublicKey:  params.ReferrerInfo.Referrer,
			IsWritable: true,
			IsSigner:   false,
		}, &solana.AccountMeta{
			PublicKey:  params.ReferrerInfo.ReferrerStats,
			IsWritable: true,
			IsSigner:   false,
		})
	}
	if order.MarketType == drift.MarketType_Spot {
		remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
			PublicKey:  p.driftClient.GetSpotMarketAccount(order.MarketIndex).Vault,
			IsWritable: false,
			IsSigner:   false,
		}, &solana.AccountMeta{
			PublicKey:  p.driftClient.GetQuoteSpotMarketAccount().Vault,
			IsWritable: false,
			IsSigner:   false,
		})
	}
	builder := jit_proxy.NewJitInstructionBuilder()
	builder.SetParams(jit_proxy.JitParams{
		TakerOrderId: params.TakerOrderId,
		MaxPosition:  params.MaxPosition.Int64(),
		MinPosition:  params.MinPosition.Int64(),
		Bid:          params.Bid.Int64(),
		Ask:          params.Ask.Int64(),
		PriceType:    *params.PriceType,
		PostOnly:     params.PostOnly,
	}).SetTakerAccount(params.TakerKey).
		SetTakerStatsAccount(params.TakerStatsKey).
		SetStateAccount(p.driftClient.GetStatePublicKey()).
		SetUserAccount(p.driftClient.GetUserAccountPublicKey(&subAccountId, nil)).
		SetUserStatsAccount(p.driftClient.GetUserStatsAccountPublicKey()).
		SetDriftProgramAccount(p.driftClient.GetProgram().GetProgramId())
	if len(remainingAccounts) > 0 {
		for _, remainingAccount := range remainingAccounts {
			builder.Append(remainingAccount)
		}
	}
	return builder.ValidateAndBuild()
}

func (p *JitProxyClient) GetCheckOrderConstraintIx(
	subAccountId *uint16,
	orderConstraints []jit_proxy.OrderConstraint,
) (solana.Instruction, error) {
	driftSubAccountId := utils.TTM[uint16](subAccountId == nil,
		func() uint16 { return p.driftClient.GetActiveSubAccountId() },
		func() uint16 { return *subAccountId },
	)
	var readablePerpMarketIndexes []uint16
	var readableSpotMarketIndexes []uint16
	for idx := 0; idx < len(orderConstraints); idx++ {
		if orderConstraints[idx].MarketType == jit_proxy.MarketTypePerp {
			readablePerpMarketIndexes = append(readablePerpMarketIndexes, orderConstraints[idx].MarketIndex)
		} else {
			readableSpotMarketIndexes = append(readableSpotMarketIndexes, orderConstraints[idx].MarketIndex)
		}
	}
	remainingAccounts := p.driftClient.GetRemainingAccounts(&types.RemainingAccountParams{
		UserAccounts:            []*drift.User{p.driftClient.GetUserAccount(&driftSubAccountId, nil)},
		ReadableSpotMarketIndex: readableSpotMarketIndexes,
		ReadablePerpMarketIndex: readablePerpMarketIndexes,
	})
	builder := jit_proxy.NewCheckOrderConstraintsInstructionBuilder()
	builder.SetConstraints(orderConstraints).
		SetUserAccount(p.driftClient.GetUserAccountPublicKey(&driftSubAccountId, nil))
	if len(remainingAccounts) > 0 {
		for _, remainingAccount := range remainingAccounts {
			builder.Append(remainingAccount)
		}
	}
	return builder.ValidateAndBuild()
}

func (p *JitProxyClient) ArbPerp(makerInfos []*go_drift.MakerInfo, marketIndex uint16, txParams *go_drift.TxParams) (*tx.TxSigAndSlot, error) {
	ix, err := p.GetArbPerpIx(makerInfos, marketIndex, nil)
	if err != nil {
		return nil, err
	}
	tx := p.driftClient.BuildTransaction([]solana.Instruction{ix}, txParams, []addresslookuptable.KeyedAddressLookupTable{})
	return p.driftClient.SendTransaction(tx, nil, false)
}

func (p *JitProxyClient) GetArbPerpIx(makerInfos []*go_drift.MakerInfo, marketIndex uint16, referrerInfo *go_drift.ReferrerInfo) (solana.Instruction, error) {
	var userAccounts []*drift.User
	for _, makerInfo := range makerInfos {
		userAccounts = append(userAccounts, makerInfo.MakerUserAccount)
	}

	remainingAccounts := p.driftClient.GetRemainingAccounts(&types.RemainingAccountParams{
		UserAccounts:              userAccounts,
		WritablePerpMarketIndexes: []uint16{marketIndex},
	})

	for _, makerInfo := range makerInfos {
		remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
			PublicKey:  makerInfo.Maker,
			IsWritable: true,
			IsSigner:   false,
		}, &solana.AccountMeta{
			PublicKey:  makerInfo.MakerStats,
			IsWritable: true,
			IsSigner:   false,
		})
	}

	if referrerInfo != nil {
		referrerIsMaker := slices.ContainsFunc(makerInfos, func(makerInfo *go_drift.MakerInfo) bool {
			return makerInfo.Maker.Equals(referrerInfo.Referrer)
		})
		if !referrerIsMaker {
			remainingAccounts = append(remainingAccounts, &solana.AccountMeta{
				PublicKey:  referrerInfo.Referrer,
				IsWritable: true,
				IsSigner:   false,
			}, &solana.AccountMeta{
				PublicKey:  referrerInfo.ReferrerStats,
				IsWritable: true,
				IsSigner:   false,
			})
		}
	}
	builder := jit_proxy.NewArbPerpInstructionBuilder()
	builder.SetMarketIndex(marketIndex).
		SetStateAccount(p.driftClient.GetStatePublicKey()).
		SetUserAccount(p.driftClient.GetUserAccountPublicKey(nil, nil)).
		SetUserStatsAccount(p.driftClient.GetUserStatsAccountPublicKey()).
		SetDriftProgramAccount(p.driftClient.GetProgram().GetProgramId())

	if len(remainingAccounts) > 0 {
		for _, remainingAccount := range remainingAccounts {
			builder.Append(remainingAccount)
		}
	}
	return builder.ValidateAndBuild()
}
