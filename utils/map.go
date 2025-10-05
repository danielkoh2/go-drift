package utils

import "github.com/gagliardetto/solana-go"

func MapValues[K string | solana.PublicKey | int | int32 | int64 | uint | uint16 | uint32 | uint64, T any](m map[K]T) []T {
	var values []T
	for _, value := range m {
		values = append(values, value)
	}
	return values
}

func MapKeys[K string | solana.PublicKey | int | int32 | int64 | uint | uint16 | uint32 | uint64, T any](m map[K]T) []K {
	var keys []K
	for key, _ := range m {
		keys = append(keys, key)
	}
	return keys
}

func MapFunc[K string | solana.PublicKey | int | int32 | int64 | uint | uint16 | uint32 | uint64, T any, F any](m map[K]T, f func(e T) F) []F {
	var values []F
	for _, value := range m {
		values = append(values, f(value))
	}
	return values
}

func MapHas[K string | solana.PublicKey | int | int32 | int64 | uint | uint16 | uint32 | uint64, T any](m map[K]T, k K) bool {
	if m == nil {
		return false
	}
	_, ok := m[k]
	return ok
}
