package dlob

import (
	"context"
	go_drift "driftgo"
	"driftgo/dlob/types"
	drift "driftgo/drift/types"
	"driftgo/events"
	driftlib "driftgo/lib/drift"
	"driftgo/lib/event"
	"github.com/gagliardetto/solana-go"
	"slices"
	"sync"
	"time"
)

type DLOBSubscriber struct {
	driftClient                    drift.IDriftClient
	dlobSource                     types.IDLOBSource
	slotSource                     types.ISlotSource
	updateFrequency                int64
	dlob                           types.IDLOB
	eventEmitter                   *event.EventEmitter
	isSubscribed                   bool
	cancel                         func()
	mxState                        *sync.RWMutex
	mxEvents                       *sync.RWMutex
	eventCache                     []*events.WrappedEvent
	maxEventCache                  int
	timeoutEventCacheMs            int64
	disableChangeByUserOrderUpdate bool
	disableHandleOrderRecord       bool
}

func CreateDLOBSubscriber(config types.DLOBSubscriptionConfig) *DLOBSubscriber {
	return &DLOBSubscriber{
		driftClient:                    config.DriftClient,
		dlobSource:                     config.DlobSource,
		slotSource:                     config.SlotSource,
		updateFrequency:                config.UpdateFrequency,
		dlob:                           NewDLOB(),
		eventEmitter:                   go_drift.EventEmitter(),
		mxState:                        new(sync.RWMutex),
		mxEvents:                       new(sync.RWMutex),
		maxEventCache:                  10,
		timeoutEventCacheMs:            5,
		disableChangeByUserOrderUpdate: config.DisableChangeByUserOrderUpdate,
		disableHandleOrderRecord:       config.DisableHandleOrderRecord,
		//disableHandleOrderRecord:       true,
	}
}

func (p *DLOBSubscriber) GetSlotSource() types.ISlotSource {
	return p.slotSource
}

func (p *DLOBSubscriber) GetDlobSource() types.IDLOBSource {
	return p.dlobSource
}

func (p *DLOBSubscriber) Subscribe() {
	if p.isSubscribed {
		return
	}
	p.updateDLOB()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Duration(p.updateFrequency) * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				//p.updateDLOB()
			case <-ctx.Done():
				goto END
			}
		}
	END:
	}(ctx)
	p.isSubscribed = true
	if !p.disableChangeByUserOrderUpdate {
		go_drift.EventEmitter().On(go_drift.EventLibUserOrderUpdated, func(data ...interface{}) {
			userAccountKey := data[0].(string)
			userAccount := data[1].(*driftlib.User)
			slot := data[2].(uint64)
			txnHash := data[3].(string)
			//startTime := time.Now().UnixMilli()
			p.updateDLOBByUser(solana.MPK(userAccountKey), userAccount, slot, txnHash)
			//fmt.Printf("DLOB updated by user (%s): duration %d ms\n", userAccountKey, time.Now().UnixMilli()-startTime)
		})
	}
	if !p.disableHandleOrderRecord {
		var lastHandled int64
		go_drift.EventEmitter().On("newEvent", func(data ...interface{}) {
			wrappedEvents := data[0].([]*events.WrappedEvent)
			if p.dlob == nil {
				return
			}
			filteredEvents := slices.DeleteFunc(wrappedEvents, func(event *events.WrappedEvent) bool {
				return !(event.EventType == events.EventTypeOrderRecord || event.EventType == events.EventTypeOrderActionRecord)
			})
			if len(filteredEvents) > 0 && p.pendEvent(filteredEvents...) {
				lastHandled = time.Now().UnixMilli()
			}
			//	p.dlob.HandleOrderRecord(wrappedEvent.Data.(*driftlib.OrderRecord), wrappedEvent.Slot)
			//} else if wrappedEvent.EventType == events.EventTypeOrderActionRecord {
			//	p.dlob.HandleOrderActionRecord(wrappedEvent.Data.(*driftlib.OrderActionRecord), wrappedEvent.Slot)

		})
		go func(ctx context.Context) {
			ticker := time.NewTicker(1 * time.Millisecond)
			for {
				select {
				case <-ticker.C:
					if lastHandled == 0 || time.Now().UnixMilli()-lastHandled > p.timeoutEventCacheMs {
						p.handleEvents()
						lastHandled = time.Now().UnixMilli()
					}
				case <-ctx.Done():
					goto END
				}
			}
		END:
		}(ctx)
	}
}

func (p *DLOBSubscriber) pendEvent(wrappedEvent ...*events.WrappedEvent) bool {
	p.mxEvents.Lock()
	p.eventCache = append(p.eventCache, wrappedEvent...)
	p.mxEvents.Unlock()
	if len(p.eventCache) >= p.maxEventCache {
		p.handleEvents()
		return true
	}
	return false
}

func (p *DLOBSubscriber) handleEvents() {
	if len(p.eventCache) == 0 {
		return
	}

	p.mxEvents.Lock()
	eventCache := p.eventCache[0:]
	p.eventCache = p.eventCache[:0]
	p.mxEvents.Unlock()

	if p.dlob != nil && len(eventCache) == 0 {
		return
	}
	p.dlob.HandleOrderEvents(eventCache)
	p.eventEmitter.Emit(go_drift.EventLibNewOrderRecords, p.dlob)
	for _, wrappedEvent := range eventCache {
		p.eventEmitter.Emit(go_drift.EventLibOrderEvent, wrappedEvent)
	}
	//fmt.Println("order records handled : ", len(p.eventCache))
}

func (p *DLOBSubscriber) updateDLOBByUser(userAccountKey solana.PublicKey, userAccount *driftlib.User, slot uint64, txnHash string) {
	if !p.isSubscribed || p.dlob == nil {
		return
	}
	p.mxState.Lock()
	p.dlob.UpdateByUser(userAccountKey, userAccount, slot)
	p.mxState.Unlock()
}

func (p *DLOBSubscriber) updateDLOB() {
	defer p.mxState.Unlock()
	dlob := p.dlobSource.GetDLOB(p.slotSource.GetSlot())
	p.mxState.Lock()
	p.dlob = dlob
}

func (p *DLOBSubscriber) GetDLOB() types.IDLOB {
	defer p.mxState.RUnlock()
	p.mxState.RLock()
	return p.dlob
}

func (p *DLOBSubscriber) Unsubscribe() {
	if !p.isSubscribed {
		return
	}
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.isSubscribed = false
}
