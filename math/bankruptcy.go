package math

import (
	"driftgo/drift/types"
	"driftgo/lib/drift"
)

func IsUserBankrupt(user types.IUser) bool {
	userAccount := user.GetUserAccount()
	hasLiability := false

	for idx := 0; idx < len(userAccount.SpotPositions); idx++ {
		position := &userAccount.SpotPositions[idx]
		if position.ScaledBalance > 0 {
			if position.BalanceType == drift.SpotBalanceType_Deposit {
				return false
			}
			if position.BalanceType == drift.SpotBalanceType_Borrow {
				hasLiability = true
			}
		}
	}

	for idx := 0; idx < len(userAccount.PerpPositions); idx++ {
		position := &userAccount.PerpPositions[idx]
		if position.BaseAssetAmount != 0 ||
			position.QuoteAssetAmount > 0 ||
			HasOpenOrders(position) ||
			position.LpShares > 0 {
			return false
		}

		if position.QuoteAssetAmount < 0 {
			hasLiability = true
		}
	}
	return hasLiability
}
