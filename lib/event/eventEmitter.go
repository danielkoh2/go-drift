package event

import (
	"driftgo/utils"
	"slices"
	"sync"
)

type CallbackItem struct {
	Id        string
	Priority  int
	IsOnetime bool
	Callback  Callback
}
type Callback func(object ...interface{})

type EventEmitter struct {
	callbacks map[string][]CallbackItem
	mxState   *sync.RWMutex
}

func CreateEventEmitter() *EventEmitter {
	return &EventEmitter{
		callbacks: make(map[string][]CallbackItem),
		mxState:   new(sync.RWMutex),
	}
}

func (s *EventEmitter) addHandler(event string, callbackItem CallbackItem) {
	defer s.mxState.Unlock()
	s.mxState.Lock()
	_, exists := s.callbacks[event]
	if !exists {
		s.callbacks[event] = make([]CallbackItem, 0)
		s.callbacks[event] = append(s.callbacks[event], callbackItem)
	} else {
		var idx int
		size := len(s.callbacks[event])
		for idx = 0; idx < size; idx++ {
			if s.callbacks[event][idx].Priority > callbackItem.Priority {
				break
			}
		}
		if idx < size {
			s.callbacks[event] = append(s.callbacks[event][:idx+1], s.callbacks[event][idx:]...)
			s.callbacks[event][idx] = callbackItem
		} else {
			s.callbacks[event] = append(s.callbacks[event], callbackItem)
		}

	}

}
func (s *EventEmitter) On(event string, callback Callback, priorityArr ...int) string {
	priority := 100
	if len(priorityArr) > 0 {
		priority = priorityArr[0]
	}
	id := utils.GenerateIdentity()
	s.addHandler(event, CallbackItem{Callback: callback, Priority: priority, Id: id})
	return id
}

func (s *EventEmitter) Once(event string, callback Callback, priorityArr ...int) string {
	priority := 100
	if len(priorityArr) > 0 {
		priority = priorityArr[0]
	}
	id := utils.GenerateIdentity()
	s.addHandler(event, CallbackItem{Callback: callback, Priority: priority, Id: id, IsOnetime: true})
	return id
}

func (s *EventEmitter) Off(event string, callbackIds ...string) {
	defer s.mxState.Unlock()
	s.mxState.Lock()
	if len(callbackIds) == 0 {
		delete(s.callbacks, event)
	} else {
		callbacks, exists := s.callbacks[event]
		if exists {
			var newCallbacks []CallbackItem
			for _, cb := range callbacks {
				if !slices.Contains(callbackIds, cb.Id) {
					newCallbacks = append(newCallbacks, cb)
				}
			}
			s.callbacks[event] = newCallbacks
		}
	}

}

func (s *EventEmitter) Emit(event string, object ...interface{}) {
	s.mxState.RLock()

	callbacks, exists := s.callbacks[event]
	if !exists {
		s.mxState.RUnlock()
		return
	}
	var removed []string
	for _, callback := range callbacks {
		go (callback.Callback)(object...)
		if callback.IsOnetime {
			removed = append(removed, callback.Id)
		}
	}
	s.mxState.RUnlock()
	if len(removed) > 0 {
		s.Off(event, removed...)
	}
}
