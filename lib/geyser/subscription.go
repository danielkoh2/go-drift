package geyser

import (
	solana_geyser_grpc "driftgo/lib/rpcpool/solana-geyser-grpc"
	"io"
)

type Subscription struct {
	Client       solana_geyser_grpc.GeyserClient
	Stream       solana_geyser_grpc.Geyser_SubscribeClient
	Cancel       func()
	Handler      *SubscribeEvents
	IsSubscribed bool
	Requests     []*solana_geyser_grpc.SubscribeRequest
}

func (p *Subscription) Request(request *solana_geyser_grpc.SubscribeRequest) error {
	p.Requests = append(p.Requests, request)
	return p.Stream.Send(request)
}

func (p *Subscription) Subscribe(handler *SubscribeEvents) bool {
	if p.IsSubscribed {
		return true
	}
	p.IsSubscribed = true

	p.Handler = handler
	go func() {
		//id := GenerateIdentity()
		//log.Channel("geyser").Debug("Gyser[%s] started", id)
		defer func() {
			//log.Channel("geyser").Debug("Gyser[%s] stopped", id)
			if err := recover(); err != nil {
				//spew.Dump(err)
			} else {
				if p.Stream != nil {
					_ = p.Stream.CloseSend()
				}
			}
		}()
		for {
			resp, err := p.Stream.Recv()
			if !p.IsSubscribed || p.Handler == nil {
				break
			}
			if err == io.EOF {
				if handler.Eof != nil {
					handler.Eof()
				}
				goto END
			} else if err != nil {
				if handler.Error != nil {
					handler.Error(err)
				}
				panic(err)
			} else if resp == nil {
				goto END
			} else {
				if handler.Update != nil {
					handler.Update(resp)
				}
			}
		}
	END:
	}()
	return true
}

func (p *Subscription) Unsubscribe() {
	if !p.IsSubscribed {
		return
	}
	if p.Stream != nil {
		_ = p.Stream.CloseSend()
	}
	if p.Cancel != nil {
		p.Cancel()
	}
	p.Handler = nil
	p.IsSubscribed = false
}
