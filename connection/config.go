package connection

import (
	"crypto/md5"
	"encoding/base64"
	"strings"
)

func (p *RpcConnectionConfig) Hash() string {
	t := "Endpoint:" + p.Endpoint
	if len(p.Headers) > 0 {
		for name, value := range p.Headers {
			t = t + "|" + name + ":" + value
		}
	}
	return base64.StdEncoding.EncodeToString(md5.New().Sum([]byte(t)))
}

func (p *WsConnectionConfig) Hash() string {
	t := "Endpoint:" + p.Endpoint
	if len(p.Headers) > 0 {
		for name, value := range p.Headers {
			t = t + "|" + name + ":" + strings.Join(value, " ")
		}
	}
	return base64.StdEncoding.EncodeToString(md5.New().Sum([]byte(t)))
}

func (p *GeyserConnectionConfig) Hash() string {
	t := "Endpoint:" + p.Endpoint + "|Token:" + p.Token
	if p.InSecure {
		t = t + "|HTTPS"
	}
	return base64.StdEncoding.EncodeToString(md5.New().Sum([]byte(t)))
}
