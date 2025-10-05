package types

import (
	"driftgo/anchor/types"
	"driftgo/lib/drift"
	"github.com/gagliardetto/solana-go/rpc"
)

type OracleClientCache struct {
	cache map[drift.OracleSource]IOracleClient
}

func CreateOracleClientCache() *OracleClientCache {
	return &OracleClientCache{
		cache: make(map[drift.OracleSource]IOracleClient),
	}
}

func (p *OracleClientCache) Get(
	oracleSource drift.OracleSource,
	connection *rpc.Client,
	program types.IProgram,
) IOracleClient {
	value, exists := p.cache[oracleSource]
	if exists {
		return value
	}
	p.cache[oracleSource] = GetOracleClient(oracleSource, connection, program)
	return p.cache[oracleSource]
}
