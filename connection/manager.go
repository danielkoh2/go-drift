package connection

import (
	"context"
	"crypto/md5"
	"driftgo/lib/geyser"
	"driftgo/utils"
	"encoding/base64"
	"fmt"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"regexp"
)

type Config struct {
	Host        string
	Token       string
	IsSecure    bool `yaml:"isSecure"`
	MaxReferrer int  `yaml:"maxReferrer"`
}

func (p *Config) Hash() string {
	t := fmt.Sprintf("%s://%s/%s", utils.TT(p.IsSecure, "https", "http"), p.Host, p.Token)
	return base64.StdEncoding.EncodeToString(md5.New().Sum([]byte(t)))
}

func (p *Config) GetRpcEndpoint() string {
	return fmt.Sprintf("%s://%s",
		utils.TT(p.IsSecure, "https", "http"),
		p.Host+(utils.TT(p.Token == "", "", "/"+p.Token)),
	)
}

func (p *Config) GetWsEndpoint() string {
	return fmt.Sprintf("%s://%s",
		utils.TT(p.IsSecure, "wss", "ws"),
		p.Host+(utils.TT(p.Token == "", "", "/"+p.Token)),
	)
}

func (p *Config) GetGeyserEndpoint() string {
	var portDefined = regexp.MustCompile(`^.+:\d+$`)
	host := p.Host
	if p.IsSecure && !portDefined.MatchString(host) {
		host = host + ":443"
	}
	return host
}

type Manager struct {
	configs           map[string]*Config
	rpcConnections    map[string][]*rpc.Client
	wsConnections     map[string][]*ws.Client
	geyserConnections map[string][]*geyser.Connection
}

func CreateManager() *Manager {
	return &Manager{
		configs:           make(map[string]*Config),
		rpcConnections:    make(map[string][]*rpc.Client),
		wsConnections:     make(map[string][]*ws.Client),
		geyserConnections: make(map[string][]*geyser.Connection),
	}
}

func (p *Manager) AddConfig(config Config, id ...string) {
	connectionId := config.Hash()
	if len(id) > 0 && len(id[0]) > 0 {
		connectionId = id[0]
	}
	_, exists := p.configs[connectionId]
	if !exists {
		p.configs[connectionId] = &config
	}
}

func (p *Manager) getConnectionId(id ...string) string {
	var connectionId string
	if len(id) > 0 && len(id[0]) > 0 {
		connectionId = id[0]
	}
	_, exists := p.configs[connectionId]
	if !exists {
		connectionIds := utils.MapKeys(p.configs)
		connectionId = utils.RandomElement(connectionIds)
	}
	return connectionId
}

func (p *Manager) GetRpc(id ...string) *rpc.Client {
	connectionId := p.getConnectionId(id...)
	config, _ := p.configs[connectionId]
	connectionLength := len(p.rpcConnections[connectionId])
	var connection *rpc.Client
	if connectionLength == 0 || config.MaxReferrer <= 0 || connectionLength < config.MaxReferrer {
		connection = p.CreateRpc(config)
		p.rpcConnections[connectionId] = append(p.rpcConnections[connectionId], connection)
	} else {
		connection = utils.RandomElement(p.rpcConnections[connectionId])
	}
	return connection
}

func (p *Manager) CreateRpc(config *Config) *rpc.Client {
	return rpc.New(config.GetRpcEndpoint())

}

func (p *Manager) GetWs(id ...string) *ws.Client {
	connectionId := p.getConnectionId(id...)
	config, _ := p.configs[connectionId]
	connectionLength := len(p.wsConnections[connectionId])
	var connection *ws.Client
	if connectionLength == 0 || config.MaxReferrer <= 0 || connectionLength < config.MaxReferrer {
		connection = p.CreateWs(config)
		if connection == nil {
			panic("Can not establish websocket connection")
		}
		p.wsConnections[connectionId] = append(p.wsConnections[connectionId], connection)
	} else {
		connection = utils.RandomElement(p.wsConnections[connectionId])
	}
	return connection
}

func (p *Manager) CreateWs(config *Config) *ws.Client {
	connection, err := ws.Connect(context.Background(), config.GetWsEndpoint())
	if err != nil {
		return nil
	}
	return connection
}

func (p *Manager) GetGeyser(id ...string) *geyser.Connection {
	connectionId := p.getConnectionId(id...)
	config, _ := p.configs[connectionId]
	connectionLength := len(p.geyserConnections[connectionId])
	var connection *geyser.Connection
	if connectionLength == 0 || config.MaxReferrer <= 0 || connectionLength < config.MaxReferrer {
		connection = p.CreateGeyser(config)
		if connection == nil {
			panic("Can not establish geyser connection")
		}
		p.geyserConnections[connectionId] = append(p.geyserConnections[connectionId], connection)
	} else {
		connection = utils.RandomElement(p.geyserConnections[connectionId])
	}
	return connection
}

func (p *Manager) CreateGeyser(config *Config) *geyser.Connection {
	connection := geyser.NewConnection()
	err := connection.Connect(
		geyser.ConnectionConfig{
			Identity: config.Hash(),
			Token:    config.Token,
			Endpoint: config.GetGeyserEndpoint(),
			Insecure: config.IsSecure,
		})
	if err != nil {
		return nil
	}
	return connection
}
