package addresses

import (
	"encoding/binary"
	"github.com/gagliardetto/solana-go"
)

func GetDriftStateAccountPublicKeyAndNonce(
	programId solana.PublicKey,
) (solana.PublicKey, uint8) {
	address, bumpSeed, err := solana.FindProgramAddress(
		[][]byte{[]byte("drift_state")},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}, 0
	}
	return address, bumpSeed
}

func GetDriftStateAccountPublicKey(
	programId solana.PublicKey,
) solana.PublicKey {
	address, _ := GetDriftStateAccountPublicKeyAndNonce(programId)
	return address
}

func GetUserAccountPublicKeyAndNonce(
	programId solana.PublicKey,
	authority solana.PublicKey,
	subAccountIds ...uint16,
) (solana.PublicKey, uint8) {
	var subAccountId uint16 = 0
	if len(subAccountIds) > 0 {
		subAccountId = subAccountIds[0]
	}
	seed := make([]byte, 2)
	binary.LittleEndian.PutUint16(seed, uint16(subAccountId))
	address, bumpSeed, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("user"),
			authority.Bytes(),
			seed,
		},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}, 0
	}
	return address, bumpSeed
}

func GetUserAccountPublicKey(
	programId solana.PublicKey,
	authority solana.PublicKey,
	subAccountIds ...uint16,
) solana.PublicKey {
	address, _ := GetUserAccountPublicKeyAndNonce(programId, authority, subAccountIds...)
	return address
}

func GetUserStatsAccountPublicKey(
	programId solana.PublicKey,
	authority solana.PublicKey,
) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("user_stats"), authority.Bytes()},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetPerpMarketPublicKey(
	programId solana.PublicKey,
	marketIndex uint16,
) solana.PublicKey {
	index := make([]byte, 2)
	binary.LittleEndian.PutUint16(index, marketIndex)
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("perp_market"), index},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetSpotMarketPublicKey(
	programId solana.PublicKey,
	marketIndex uint16,
) solana.PublicKey {
	index := make([]byte, 2)
	binary.LittleEndian.PutUint16(index, marketIndex)
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("spot_market"), index},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetSpotMarketVaultPublicKey(
	programId solana.PublicKey,
	marketIndex uint16,
) solana.PublicKey {
	index := make([]byte, 2)
	binary.LittleEndian.PutUint16(index, marketIndex)
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("spot_market_vault"), index},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetInsuranceFundVaultPublicKey(
	programId solana.PublicKey,
	marketIndex uint16,
) solana.PublicKey {
	index := make([]byte, 2)
	binary.LittleEndian.PutUint16(index, marketIndex)
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("insurance_fund_vault"), index},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetInsuranceFundStakeAccountPublicKey(
	programId solana.PublicKey,
	authority solana.PublicKey,
	marketIndex uint16,
) solana.PublicKey {
	index := make([]byte, 2)
	binary.LittleEndian.PutUint16(index, marketIndex)
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("insurance_fund_stake"), authority.Bytes(), index},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetDriftSignerPublicKey(programId solana.PublicKey) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("drift_signer")},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetSerumOpenOrdersPublicKey(
	programId solana.PublicKey,
	market solana.PublicKey,
) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("serum_open_orders"), market.Bytes()},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetSerumSignerPublicKey(
	programId solana.PublicKey,
	market solana.PublicKey,
	nonce uint64,
) solana.PublicKey {
	seed := make([]byte, 8)
	binary.LittleEndian.PutUint64(seed, nonce)

	address, err := solana.CreateProgramAddress(
		[][]byte{market.Bytes(), seed},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetSerumFulfillmentConfigPublicKey(
	programId solana.PublicKey,
	market solana.PublicKey,
) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("serum_fulfillment_config"), market.Bytes()},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetPhoenixFulfillmentConfigPublicKey(
	programId solana.PublicKey,
	market solana.PublicKey,
) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("phoenix_fulfillment_config"), market.Bytes()},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetReferrerNamePublicKey(
	programId solana.PublicKey,
	nameBuffer []byte,
) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("referrer_name"), nameBuffer},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}

func GetProtocolIfSharesTransferConfigPublicKey(
	programId solana.PublicKey,
) solana.PublicKey {
	address, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("if_shares_transfer_config")},
		programId,
	)
	if err != nil {
		return solana.PublicKey{}
	}
	return address
}
