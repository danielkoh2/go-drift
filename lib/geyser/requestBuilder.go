package geyser

import (
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go/rpc"
)

type AccountParams struct {
	SubAccounts     []string
	ProgramAccounts []string
	Filters         []rpc.RPCFilter
}

type TransactionParams struct {
	Vote            bool
	Failed          bool
	AccountInclude  []string
	AccountExclude  []string
	AccountRequired []string
}

type SignatureParams struct {
	Signature string
}

type BlocksParams struct{}

type BlockMetaParams struct{}

type SlotParams struct{}

type SubscriptionRequestBuilder struct {
	AccountParams     *AccountParams
	TransactionParams *TransactionParams
	SignatureParams   *SignatureParams
	BlocksParams      *BlocksParams
	BlockMetaParams   *BlockMetaParams
	SlotParams        *SlotParams
	Commitment        *solana_geyser_grpc.CommitmentLevel
	Filters           []rpc.RPCFilter
}

func NewSubscriptionRequestBuilder() *SubscriptionRequestBuilder {
	return &SubscriptionRequestBuilder{}
}

func (p *SubscriptionRequestBuilder) SetCommitment(commitment *rpc.CommitmentType) *SubscriptionRequestBuilder {
	if commitment != nil {
		var level solana_geyser_grpc.CommitmentLevel
		switch *commitment {
		case rpc.CommitmentConfirmed:
			level = solana_geyser_grpc.CommitmentLevel_CONFIRMED
		case rpc.CommitmentProcessed:
			level = solana_geyser_grpc.CommitmentLevel_PROCESSED
		case rpc.CommitmentFinalized:
			level = solana_geyser_grpc.CommitmentLevel_FINALIZED
		default:
			return p
		}
		p.Commitment = &level
	}
	return p
}

func (p *SubscriptionRequestBuilder) AccountFilters(filters []rpc.RPCFilter) *SubscriptionRequestBuilder {
	if p.AccountParams == nil {
		p.AccountParams = &AccountParams{}
	}
	if len(filters) > 0 {
		p.AccountParams.Filters = filters
	}
	return p
}
func (p *SubscriptionRequestBuilder) Accounts(accounts ...string) *SubscriptionRequestBuilder {
	if p.AccountParams == nil {
		p.AccountParams = &AccountParams{}
	}
	p.AccountParams.SubAccounts = accounts
	return p
}

func (p *SubscriptionRequestBuilder) ProgramAccounts(accounts ...string) *SubscriptionRequestBuilder {
	if p.AccountParams == nil {
		p.AccountParams = &AccountParams{}
	}
	p.AccountParams.ProgramAccounts = accounts
	return p
}

func (p *SubscriptionRequestBuilder) Signatures(signature string) *SubscriptionRequestBuilder {
	if p.SignatureParams == nil {
		p.SignatureParams = &SignatureParams{}
	}
	p.SignatureParams.Signature = signature
	return p
}

func (p *SubscriptionRequestBuilder) Blocks() *SubscriptionRequestBuilder {
	if p.BlocksParams == nil {
		p.BlocksParams = &BlocksParams{}
	}
	return p
}

func (p *SubscriptionRequestBuilder) BlockMeta() *SubscriptionRequestBuilder {
	if p.BlockMetaParams == nil {
		p.BlockMetaParams = &BlockMetaParams{}
	}
	return p
}

func (p *SubscriptionRequestBuilder) Slots() *SubscriptionRequestBuilder {
	if p.SlotParams == nil {
		p.SlotParams = &SlotParams{}
	}
	return p
}
func (p *SubscriptionRequestBuilder) Transactions(
	accountInclude []string,
	accountExclude []string,
	accountRequired []string,
	vote bool,
	failed bool,
) *SubscriptionRequestBuilder {
	if p.TransactionParams == nil {
		p.TransactionParams = &TransactionParams{}
	}
	p.TransactionParams.Vote = vote
	p.TransactionParams.Failed = failed
	p.TransactionParams.AccountInclude = accountInclude
	p.TransactionParams.AccountExclude = accountExclude
	p.TransactionParams.AccountRequired = accountRequired
	return p
}

func (p *SubscriptionRequestBuilder) Build() *solana_geyser_grpc.SubscribeRequest {
	request := &solana_geyser_grpc.SubscribeRequest{}
	if p.AccountParams != nil {
		request.Accounts = make(map[string]*solana_geyser_grpc.SubscribeRequestFilterAccounts)
		var filters []*solana_geyser_grpc.SubscribeRequestFilterAccountsFilter
		if len(p.AccountParams.Filters) > 0 {
			for _, filter := range p.AccountParams.Filters {
				filters = append(filters, &solana_geyser_grpc.SubscribeRequestFilterAccountsFilter{
					Filter: &solana_geyser_grpc.SubscribeRequestFilterAccountsFilter_Memcmp{
						Memcmp: &solana_geyser_grpc.SubscribeRequestFilterAccountsFilterMemcmp{
							Offset: filter.Memcmp.Offset,
							Data: &solana_geyser_grpc.SubscribeRequestFilterAccountsFilterMemcmp_Bytes{
								Bytes: filter.Memcmp.Bytes,
							},
						},
					},
				})
			}
		}
		request.Accounts["account_sub"] = &solana_geyser_grpc.SubscribeRequestFilterAccounts{
			Account: p.AccountParams.SubAccounts,
			Owner:   p.AccountParams.ProgramAccounts,
			Filters: utils.TT(len(filters) > 0, filters, nil),
		}
	}
	if p.SignatureParams != nil && p.SignatureParams.Signature != "" {
		request.Transactions = make(map[string]*solana_geyser_grpc.SubscribeRequestFilterTransactions)
		tr := true
		request.Transactions["signature_sub"] = &solana_geyser_grpc.SubscribeRequestFilterTransactions{
			Vote:      &tr,
			Failed:    &tr,
			Signature: &p.SignatureParams.Signature,
		}
	}
	if p.TransactionParams != nil {
		if request.Transactions == nil {
			request.Transactions = make(map[string]*solana_geyser_grpc.SubscribeRequestFilterTransactions)
		}
		request.Transactions["transactions_sub"] = &solana_geyser_grpc.SubscribeRequestFilterTransactions{
			Vote:            &p.TransactionParams.Vote,
			Failed:          &p.TransactionParams.Failed,
			AccountInclude:  p.TransactionParams.AccountInclude,
			AccountExclude:  p.TransactionParams.AccountExclude,
			AccountRequired: p.TransactionParams.AccountRequired,
		}
	}
	if p.BlocksParams != nil {
		request.Blocks = make(map[string]*solana_geyser_grpc.SubscribeRequestFilterBlocks)
		request.Blocks["blocks"] = &solana_geyser_grpc.SubscribeRequestFilterBlocks{}
	}
	if p.SlotParams != nil {
		request.Slots = make(map[string]*solana_geyser_grpc.SubscribeRequestFilterSlots)
		request.Slots["slots_sub"] = &solana_geyser_grpc.SubscribeRequestFilterSlots{}
	}
	if p.BlockMetaParams != nil {
		request.BlocksMeta = make(map[string]*solana_geyser_grpc.SubscribeRequestFilterBlocksMeta)
		request.BlocksMeta["block_meta"] = &solana_geyser_grpc.SubscribeRequestFilterBlocksMeta{}
	}
	if p.Commitment != nil {
		request.Commitment = p.Commitment
	}
	return request
}
