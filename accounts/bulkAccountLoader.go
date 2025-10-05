package accounts

import (
	"bytes"
	"context"
	"driftgo/lib/geyser"
	"driftgo/utils"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"sync"
	"time"
)

type AccountToLoad struct {
	PublicKey solana.PublicKey
	Callbacks map[string]func([]byte, uint64)
}

const GET_MULTIPLE_ACCOUNTS_CHUNK_SIZE = 99

const oneMinute = 60 * 1000

type BulkAccountLoader struct {
	connection                    *rpc.Client
	commitment                    rpc.CommitmentType
	pollingFrequency              int64
	accountsToLoad                map[string]*AccountToLoad
	bufferAndSlotMap              map[string]*BufferAndSlot
	errorCallbacks                map[string]func(error)
	lastTimeLoadingPromiseCleared int64
	mostRecentSlot                uint64
	cancelPolling                 func()
	mxState                       *sync.RWMutex
}

func CreateBulkAccountLoader(
	connection *rpc.Client,
	commitment rpc.CommitmentType,
	pollingFrequency int64,
) *BulkAccountLoader {
	return &BulkAccountLoader{
		connection:       connection,
		commitment:       commitment,
		pollingFrequency: pollingFrequency,
		accountsToLoad:   make(map[string]*AccountToLoad),
		bufferAndSlotMap: make(map[string]*BufferAndSlot),
		errorCallbacks:   make(map[string]func(error)),
		mxState:          new(sync.RWMutex),
	}
}

func (p *BulkAccountLoader) AddAccount(
	publicKey solana.PublicKey,
	callback func([]byte, uint64),
) string {
	if publicKey.IsZero() {
		fmt.Println("Caught adding blank publickey to bulkAccountLoader")
	}

	defer p.mxState.Unlock()
	p.mxState.Lock()
	existingSize := len(p.accountsToLoad)

	callbackId := geyser.GenerateIdentity()
	existingAccountToLoad, exists := p.accountsToLoad[publicKey.String()]
	if exists {
		existingAccountToLoad.Callbacks[callbackId] = callback
	} else {
		p.accountsToLoad[publicKey.String()] = &AccountToLoad{
			PublicKey: publicKey,
			Callbacks: map[string]func([]byte, uint64){
				callbackId: callback,
			},
		}
	}

	if existingSize == 0 {
		p.StartPolling()
	}

	return callbackId
}

func (p *BulkAccountLoader) RemoveAccount(
	publicKey solana.PublicKey,
	callbackId string,
) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	existingAccountToLoad, exists := p.accountsToLoad[publicKey.String()]
	if exists {
		delete(existingAccountToLoad.Callbacks, callbackId)
		if len(existingAccountToLoad.Callbacks) == 0 {
			delete(p.bufferAndSlotMap, publicKey.String())
			delete(p.accountsToLoad, existingAccountToLoad.PublicKey.String())
		}
	}

	if len(p.accountsToLoad) == 0 {
		p.StopPolling()
	}
}

func (p *BulkAccountLoader) AddErrorCallbacks(callback func(error)) string {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	callbackId := geyser.GenerateIdentity()
	p.errorCallbacks[callbackId] = callback
	return callbackId
}

func (p *BulkAccountLoader) RemoveErrorCallbacks(callbackId string) {
	defer p.mxState.Unlock()
	p.mxState.Lock()
	delete(p.errorCallbacks, callbackId)
}

func chunks[T any](array []T, size int) [][]T {
	var chunkArray [][]T
	idx := 0
	for start := 0; start < len(array); start += size {
		chunkArray[idx] = array[start : start+size]
	}
	return chunkArray
}

func (p *BulkAccountLoader) Load() {
	for _, chunk := range chunks(chunks(utils.MapValues(p.accountsToLoad), GET_MULTIPLE_ACCOUNTS_CHUNK_SIZE), 10) {
		p.LoadChunk(chunk)
	}
}

func (p *BulkAccountLoader) LoadChunk(accountsToLoadChunks [][]*AccountToLoad) {
	if len(accountsToLoadChunks) == 0 {
		return
	}
	for _, accountsToLoadChunk := range accountsToLoadChunks {
		var keys []solana.PublicKey
		for _, accountToLoad := range accountsToLoadChunk {
			keys = append(keys, accountToLoad.PublicKey)
		}
		rpcResponse, err := p.connection.GetMultipleAccounts(
			context.TODO(),
			keys...,
		)
		if err != nil {
			continue
		}
		newSlot := rpcResponse.Context.Slot

		if newSlot > p.mostRecentSlot {
			p.mostRecentSlot = newSlot
		}
		utils.ForEach(accountsToLoadChunk, func(accountToLoad *AccountToLoad, j int) {
			if len(accountToLoad.Callbacks) == 0 {
				return
			}

			key := accountToLoad.PublicKey.String()
			oldRPCResponse, exists := p.bufferAndSlotMap[key]
			if exists && oldRPCResponse != nil && newSlot < oldRPCResponse.Slot {
				return
			}

			var newBuffer []byte
			if len(rpcResponse.Value) > j {
				newBuffer = rpcResponse.Value[j].Data.GetBinary()
			}

			if oldRPCResponse == nil {
				p.bufferAndSlotMap[key] = &BufferAndSlot{
					Buffer: newBuffer,
					Slot:   newSlot,
				}
				p.HandleAccountCallbacks(accountToLoad, newBuffer, newSlot)
				return
			}

			oldBuffer := oldRPCResponse.Buffer
			if newBuffer != nil && (oldBuffer == nil || bytes.Compare(newBuffer, oldBuffer) != 0) {
				p.bufferAndSlotMap[key] = &BufferAndSlot{
					Buffer: newBuffer,
					Slot:   newSlot,
				}
				p.HandleAccountCallbacks(accountToLoad, newBuffer, newSlot)
			}
		})
	}
}

func (p *BulkAccountLoader) HandleAccountCallbacks(
	accountToLoad *AccountToLoad,
	buffer []byte,
	slot uint64,
) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Bulk account load: error in account callback")
			fmt.Printf("account to load %s", accountToLoad.PublicKey.String())
		}
	}()
	for _, callback := range accountToLoad.Callbacks {
		callback(buffer, slot)
	}
}

func (p *BulkAccountLoader) GetBufferAndSlot(publickey solana.PublicKey) *BufferAndSlot {
	data, exists := p.bufferAndSlotMap[publickey.String()]
	if !exists {
		return nil
	}
	return data
}

func (p *BulkAccountLoader) GetSlot() uint64 {
	return p.mostRecentSlot
}

func (p *BulkAccountLoader) StartPolling() {
	if p.cancelPolling != nil {
		return
	}
	if p.pollingFrequency > 0 {
		ticker := time.NewTicker(time.Microsecond * time.Duration(p.pollingFrequency))
		ctx, cancel := context.WithCancel(context.Background())
		p.cancelPolling = cancel
		func(ctx context.Context) {
			for {
				select {
				case <-ticker.C:
					p.Load()
				case <-ctx.Done():
					return
				}
			}
		}(ctx)
	}
}

func (p *BulkAccountLoader) StopPolling() {
	if p.cancelPolling != nil {
		p.cancelPolling()
		p.cancelPolling = nil
	}
}

func (p *BulkAccountLoader) UpdatePollingFrequency(pollingFrequency int64) {
	p.StopPolling()
	p.pollingFrequency = pollingFrequency
	if len(p.accountsToLoad) > 0 {
		p.StartPolling()
	}
}
