package namespace

import (
	"context"
	go_drift "driftgo"
	"driftgo/anchor/types"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"reflect"
)

type AccountNamespace struct {
	types.IAccountNamespace
	Provider  types.IProvider
	clientMap map[string]*AccountClient
}

func CreateAccountNamespace(provider types.IProvider) *AccountNamespace {
	return &AccountNamespace{
		Provider:  provider,
		clientMap: make(map[string]*AccountClient),
	}
}
func (p *AccountNamespace) Client(t any, accountName string) *AccountClient {
	mapKey := reflect.TypeOf(t).String()
	accountClient, exists := p.clientMap[mapKey]
	if !exists {
		accountClient = &AccountClient{
			provider:    p.Provider,
			accountType: reflect.TypeOf(t),
			accountName: accountName,
		}
		p.clientMap[mapKey] = accountClient
	}
	return accountClient
}

type AccountClient struct {
	types.IAccountClient
	provider    types.IProvider
	accountType reflect.Type
	accountName string
}

func (p *AccountClient) Decode(data []byte) interface{} {
	obj := reflect.New(p.accountType).Interface()
	err := bin.NewBinDecoder(data).Decode(obj)
	if err != nil {
		return nil
	}
	return obj
}

func (p *AccountClient) FetchNullableAndContext(
	address solana.PublicKey,
	commitment rpc.CommitmentType,
) (interface{}, *rpc.Context) {
	accountInfo, rpcContext := p.getAccountInfoAndContext(address, commitment)
	obj := reflect.New(p.accountType).Interface()
	err := bin.NewBinDecoder(accountInfo.Data.GetBinary()).Decode(obj)
	if err != nil {
		fmt.Println("Fetch account failed ", obj, err)
		return nil, rpcContext
	}
	return obj, rpcContext
}

func (p *AccountClient) Fetch(
	address solana.PublicKey,
	commitment rpc.CommitmentType,
) interface{} {
	data, _ := p.FetchNullableAndContext(address, commitment)
	return data
}

func (p *AccountClient) FetchAndContext(
	address solana.PublicKey,
	commitment rpc.CommitmentType,
) (interface{}, *rpc.Context) {
	data, rpcContext := p.FetchNullableAndContext(address, commitment)
	if data == nil {
		return nil, nil
	}
	return data, rpcContext
}

func (p *AccountClient) FetchMultiple(
	addresses []solana.PublicKey,
	commitment rpc.CommitmentType,
) []interface{} {
	accounts, _ := p.FetchMultipleAndContext(addresses, commitment)
	return accounts
}

func (p *AccountClient) FetchMultipleAndContext(
	addresses []solana.PublicKey,
	commitment rpc.CommitmentType,
) ([]interface{}, *rpc.Context) {
	var accounts []interface{}
	chunksize := 100
	total := len(addresses)
	var rpcContext *rpc.Context
	for offset := 0; offset < total; offset += chunksize {
		length := min(chunksize, total-offset)
		ret, err := p.provider.GetConnection().GetMultipleAccountsWithOpts(
			context.TODO(),
			addresses[offset:length],
			&rpc.GetMultipleAccountsOpts{
				Commitment: commitment,
			},
		)
		if err != nil {
			continue
		}
		rpcContext = &ret.Context
		for _, account := range ret.Value {
			obj := reflect.New(p.accountType).Interface()
			err = bin.NewBinDecoder(account.Data.GetBinary()).Decode(obj)
			if err == nil {
				accounts = append(accounts, obj)
			}
		}
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	return accounts, rpcContext
}
func (p *AccountClient) All(filters []rpc.RPCFilter) []interface{} {
	filters = append(filters, go_drift.GetAccountFilter(p.accountName))
	connection := go_drift.Connec ///demo testing....
	accountInfos, err := connection.GetProgramAccountsWithOpts(
		//accountInfos, err := p.provider.GetConnection().GetProgramAccountsWithOpts(
		context.TODO(),
		p.provider.GetProgram().GetProgramId(),
		&rpc.GetProgramAccountsOpts{
			Filters:    filters,
			Commitment: rpc.CommitmentFinalized,
		},
	)

	if err != nil {
		return nil
	}
	var accounts []interface{}
	for _, accountInfo := range accountInfos {
		obj := reflect.New(p.accountType).Interface()
		err = bin.NewBinDecoder(accountInfo.Account.Data.GetBinary()).Decode(obj)
		if err == nil {
			accounts = append(accounts, obj)
		}
	}
	return accounts
}

func (p *AccountClient) getAccountInfoAndContext(
	address solana.PublicKey,
	commitment rpc.CommitmentType,
) (*rpc.Account, *rpc.Context) {
	accountInfo, rpcContext, err := p.provider.GetConnection().GetAccountInfoWithRpcContext(
		context.TODO(),
		address,
		&rpc.GetAccountInfoOpts{
			Commitment: commitment,
		},
	)
	if err != nil {
		return nil, nil
	}
	return accountInfo, &rpcContext.Context
}
