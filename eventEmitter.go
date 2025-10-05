package go_drift

import "driftgo/lib/event"

var eventEmitter *event.EventEmitter = nil

func EventEmitter() *event.EventEmitter {
	if eventEmitter == nil {
		eventEmitter = event.CreateEventEmitter()
	}
	return eventEmitter
}
