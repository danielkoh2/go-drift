package accounts

import (
	"context"
	"driftgo/addresses"
	"driftgo/anchor"
	"driftgo/lib/drift"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func FetchUserAccounts(
	connection *rpc.Client,
	program *anchor.Program,
	authority solana.PublicKey,
	limits ...uint16,
) []*drift.User {
	limit := uint16(8)
	if len(limits) > 0 {
		limit = limits[0]
	}
	var userAccountPublicKeys []solana.PublicKey
	for i := uint16(0); i < limit; i++ {
		userAccountPublicKeys = append(userAccountPublicKeys, addresses.GetUserAccountPublicKey(program.GetProgramId(), authority, i))
	}
	return FetchUserAccountsUsingKeys(connection, program, userAccountPublicKeys)
}

func FetchUserAccountsUsingKeys(
	connection *rpc.Client,
	program *anchor.Program,
	userAccountPublicKeys []solana.PublicKey,
) []*drift.User {
	out, err := connection.GetMultipleAccounts(
		context.TODO(),
		userAccountPublicKeys...,
	)
	if err != nil {
		return []*drift.User{}
	}
	var users []*drift.User
	decoder := bin.NewBinDecoder([]byte{})
	for _, accountInfo := range out.Value {
		var user drift.User
		decoder.Reset(accountInfo.Data.GetBinary())
		if decoder.Decode(&user) != nil {
			continue
		}
		users = append(users, &user)
	}
	return users
}

func FetchUserStatsAccount(
	connection *rpc.Client,
	programId solana.PublicKey,
	authority solana.PublicKey,
) *drift.State {
	userStatsPublicKey := addresses.GetUserStatsAccountPublicKey(programId, authority)
	accountInfo, err := connection.GetAccountInfo(context.TODO(), userStatsPublicKey)
	if err != nil || accountInfo == nil {
		return nil
	}
	userStats := &drift.State{}
	if bin.NewBinDecoder(accountInfo.GetBinary()).Decode(userStats) != nil {
		return nil
	}
	return userStats
}
