package orderSubscriber

import (
	"context"
	drift "driftgo"
	"driftgo/accounts"
	"driftgo/dlob"
	"driftgo/drift/types"
	driftlib "driftgo/lib/drift"
	"driftgo/lib/event"
	"driftgo/utils"
	"encoding/binary"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type OrderSubscriber struct {
	DriftClient  types.IDriftClient
	userAccounts map[string]*accounts.DataAndSlot[*driftlib.User]
	subscription *GeyserSubscription
	commitment   rpc.CommitmentType
	eventEmitter *event.EventEmitter

	DecodeFn        func(name string, data []byte) *driftlib.User
	RecentBlockHash solana.Hash

	RecentSlot     uint64
	mostRecentSlot uint64
	decodeData     bool
}

func CreateOrderSubscriber(config OrderSubscriberConfig) *OrderSubscriber {
	commitment := utils.TTM[rpc.CommitmentType](config.Commitment == nil, rpc.CommitmentProcessed, func() rpc.CommitmentType { return *config.Commitment })
	subscriber := &OrderSubscriber{
		DriftClient:  config.DriftClient,
		eventEmitter: drift.EventEmitter(),
		commitment:   commitment,
		userAccounts: make(map[string]*accounts.DataAndSlot[*driftlib.User]),
	}
	decoded := true
	subscriber.subscription = CreateGeyserSubscription(
		subscriber,
		commitment,
		config.SkipInitialLoad,
		config.ResubTimeoutMs,
		config.ResyncIntervalMs,
		&decoded,
	)
	//if config.FastDecode {
	subscriber.DecodeFn = func(name string, data []byte) *driftlib.User {
		var user driftlib.User
		err := bin.NewBinDecoder(data).Decode(&user)
		if err != nil {
			return nil
		}
		return &user
	}
	//} else {
	//}
	return subscriber
}

func (p *OrderSubscriber) GetUserAccounts() map[string]*accounts.DataAndSlot[*driftlib.User] {
	return p.userAccounts
}

func (p *OrderSubscriber) Subscribe() {
	p.subscription.Subscribe()
}

func (p *OrderSubscriber) Fetch() {
	accounts, err := p.DriftClient.GetProgram().GetProvider().GetConnection().GetProgramAccountsWithOpts(
		context.TODO(),
		p.DriftClient.GetProgram().GetProgramId(),
		&rpc.GetProgramAccountsOpts{
			Commitment: p.DriftClient.GetOpts().Commitment,
			Filters: []rpc.RPCFilter{
				drift.GetUserFilter(),
				drift.GetUserWithOrderFilter(),
			},
		})
	if err != nil {
		return
	}
	programAccountSet := make(map[string]bool)
	for _, programAccount := range accounts {
		key := programAccount.Pubkey.String()
		programAccountSet[key] = true
		p.TryUpdateUserAccount(
			key,
			"buffer",
			programAccount.Account.Data.GetBinary(),
			p.RecentSlot,
		)
	}
}

func (p *OrderSubscriber) TryUpdateUserAccount(
	key string,
	dataType string,
	data interface{},
	slot uint64,
) {
	if p.mostRecentSlot == 0 || slot > p.mostRecentSlot {
		p.mostRecentSlot = slot
	}
	pubkey, _ := solana.PublicKeyFromBase58(key)
	p.eventEmitter.Emit(
		"updateReceived",
		pubkey,
		slot,
		dataType,
	)

	slotAndUserAccount, exists := p.userAccounts[key]
	if !exists || slotAndUserAccount == nil || slotAndUserAccount.Slot <= slot {
		var userAccount *driftlib.User
		if dataType == "buffer" {
			buffer := data.([]byte)
			newLastActiveSlot := binary.LittleEndian.Uint64(buffer[4328 : 4328+8])
			if slotAndUserAccount != nil && slotAndUserAccount.Data.LastActiveSlot > newLastActiveSlot {
				return
			}
			userAccount = p.DecodeFn("User", buffer)
		} else {
			userAccount = data.(*driftlib.User)
		}
		p.eventEmitter.Emit("userUpdated", pubkey, userAccount, slot)

		var newOrders []*driftlib.Order
		var filterSlot uint64 = 0
		if slotAndUserAccount != nil {
			filterSlot = slotAndUserAccount.Slot
		}

		for _, order := range userAccount.Orders {
			if order.Slot > filterSlot && order.Slot <= slot {
				newOrders = append(newOrders, &order)
			}
		}

		if len(newOrders) > 0 {
			p.eventEmitter.Emit("orderCreated", pubkey, userAccount, newOrders, slot)
		}
		if userAccount.HasOpenOrder {
			p.userAccounts[key] = &accounts.DataAndSlot[*driftlib.User]{
				Data: userAccount,
				Slot: slot,
			}
		} else {
			delete(p.userAccounts, key)
		}
	}
}

func (p *OrderSubscriber) GetDLOB(slot uint64) *dlob.DLOB {
	dlob := dlob.NewDLOB()
	for key, dataAndSlot := range p.userAccounts {
		for _, order := range dataAndSlot.Data.Orders {
			dlob.InsertOrder(&order, key, slot)
		}
	}
	return dlob
}

func (p *OrderSubscriber) GetSlot() uint64 {
	return p.mostRecentSlot
}

func (p *OrderSubscriber) Unsubscribe() {
	p.subscription.Unsubscribe()
}
