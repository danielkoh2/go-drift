package addresses

import (
	"fmt"
	"github.com/gagliardetto/solana-go"
)

var CACHE = make(map[string]solana.PublicKey)

func GetMarketAddress(programId solana.PublicKey, marketIndex uint16) solana.PublicKey {
	cacheKey := fmt.Sprintf("%s-%d", programId.String(), marketIndex)
	address, exists := CACHE[cacheKey]
	if exists {
		return address
	}
	publicKey := GetPerpMarketPublicKey(programId, marketIndex)
	CACHE[cacheKey] = publicKey
	return publicKey
}
