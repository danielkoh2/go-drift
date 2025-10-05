package events

import (
	go_drift "driftgo"
	"driftgo/anchor/types"
	"driftgo/lib/event"
	"driftgo/lib/geyser"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	"slices"
)

type EventSubscriber struct {
	address           solana.PublicKey
	logProvider       ILogProvider
	eventEmitter      *event.EventEmitter
	lastSeenSlot      uint64
	lastSeenBlockTime int64
	lastSeenTxSig     string
	eventTypes        []EventType
}

func CreateEventSubscriber(
	connection *geyser.Connection,
	program types.IProgram,
	options *EventSubscriptionOptions,
) *EventSubscriber {
	if options == nil {
		options = &DefaultEventSubscriptionOptions
	}
	address := utils.TT(options.Address.IsZero(), program.GetProgramId(), options.Address)
	return &EventSubscriber{
		eventTypes:        options.EventTypes,
		address:           utils.TT(options.Address.IsZero(), program.GetProgramId(), options.Address),
		eventEmitter:      go_drift.EventEmitter(),
		lastSeenSlot:      0,
		lastSeenBlockTime: 0,
		lastSeenTxSig:     "",
		logProvider:       CreateGeyserLogProvider(connection, address, *options.Commitment, options.LogProviderConfig.ResubTimeoutMs),
	}
}

func (p *EventSubscriber) Subscribe() bool {
	if p.logProvider.IsSubscribed() {
		return true
	}
	p.logProvider.Subscribe(func(txSig solana.Signature, slot uint64, logs []string, mostRecentBlockTime int64) {
		p.handleTxLogs(txSig, slot, logs, mostRecentBlockTime)
	})
	return true
}

func (p *EventSubscriber) handleTxLogs(txSig solana.Signature, slot uint64, logs []string, mostRecentBlockTime int64) {
	p.eventEmitter.Emit("newTransactionLogs", txSig, slot, logs)
	wrappedEvents := p.parseEventsFromLogs(txSig, slot, logs)
	if len(wrappedEvents) > 0 {
		p.eventEmitter.Emit("newEvent", wrappedEvents, txSig, logs)
	}
	if p.lastSeenSlot == 0 || slot > p.lastSeenSlot {
		p.lastSeenSlot = slot
		p.lastSeenTxSig = txSig.String()
	}
	if p.lastSeenBlockTime == 0 || mostRecentBlockTime > p.lastSeenBlockTime {
		p.lastSeenBlockTime = mostRecentBlockTime
	}
}

func (p *EventSubscriber) Unsubscribe() {
	p.logProvider.Unsubscribe()
}

func (p *EventSubscriber) parseEventsFromLogs(txSig solana.Signature, slot uint64, logs []string) []*WrappedEvent {
	var records []*WrappedEvent
	events := ParseLogs(logs, p.address.String())
	for _, event := range events {
		if len(p.eventTypes) == 0 || slices.Contains(p.eventTypes, event.EventType) {
			records = append(records, &WrappedEvent{
				Event: *event,
				TxSig: txSig,
				Slot:  slot,
			})
		}
	}
	return records
}
