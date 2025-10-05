package userMap

import (
	"bytes"
	solana2 "driftgo/lib/solana"
	"encoding/base64"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/klauspost/compress/zstd"
	"log"
	"math/big"
)

func DecodeData(data string) ([]byte, error) {
	rawData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Fatal("base64 decode err", err)
	}
	decorder, err := zstd.NewReader(bytes.NewReader(rawData))
	if err != nil {
		log.Fatal("zstd.NewReaderr err", err)
	}
	defer decorder.Close()
	decoded, err := decorder.DecodeAll(rawData, nil)
	if err != nil {
		log.Fatal("decorder.DecodeAll err", err)
	}
	return decoded, nil
}

func GetTestProgramAccount() (solana2.GetProgramAccountsContextResult, error) {
	base64Data1 := "KLUv/WQYEDUGAAQIn3Vf4++XOuxfHQbmBwlqUzPmCGk5DR8ZxbrvP3+QwfK2ZNIVf32HQABEcmlmdCBMaXF1aWRpdHkgUHJvdmlkZXIgRfISNAHd+GAJAGiJAZowq///////YK2gGXz///8HhIfOAgMBAR4VdGXXqLQWUu1zGt52+4CIJQ4AAAAAAgAZAAGEAt8bvnzf8Jh1QwQAJsAIsKgXwAd0eEIFfUAhOlABDgqWBnWg18jaHxAMAGpYgQGAd+c1AKWgGDDC1eHcERqnB7Ql/m0="
	base64Data2 := "KLUv/WQYEJ0GADQIn3Vf4++XOuyVbmXHuVHVeetNivj7Y08wqmt5iCGT/CO5k+U7irmgEQBNYWluIEFjY291bnQg+nDBegWrMS1DAe5O3xPwPm0QAAAAAAFZO6HH/////xX+/SUCus9x+wyA/Vr4EwEBTE8gOpEGlL3/////8Nj/nLmH0O2DxxMPHth782ceAICj/sB+IBmw8nYhE1zrAU2uAT4EHRAq2AEKg4AaAAdAA70VAkC6Y8AJIN2hwUmCiQDSdVkMh+RUuBlfVBwAop/3wkCMkGi45IfGlQELQi+d"

	// Decode Base64 to raw bytes
	data1, err := DecodeData(base64Data1)
	if err != nil {
		panic(err)
	}
	data2, err := DecodeData(base64Data2)
	if err != nil {
		panic(err)
	}
	rentEpoch := new(big.Int)
	rentEpoch.SetString("18446744073709551615", 10)
	responseValue := []*rpc.KeyedAccount{
		{
			Pubkey: solana.MustPublicKeyFromBase58("7P34csMzr4yJzUmQH95gVXb8dHharUyRVcnxqQFZjKex"),
			Account: &rpc.Account{
				Lamports:   382547840,
				Owner:      solana.MustPublicKeyFromBase58("dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH"),
				Data:       rpc.DataBytesOrJSONFromBytes(data1),
				Executable: false,
				RentEpoch:  rentEpoch,
			},
		},
		{
			Pubkey: solana.MustPublicKeyFromBase58("7P5uZts6mgyen9VXr9AaZ2PTX4Zs5TDrzhfwvPLcaSqS"),
			Account: &rpc.Account{
				Lamports:   40763390,
				Owner:      solana.MustPublicKeyFromBase58("dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH"),
				Data:       rpc.DataBytesOrJSONFromBytes(data2),
				Executable: false,
				RentEpoch:  rentEpoch,
			},
		},
	}
	response := solana2.GetProgramAccountsContextResult{
		Context: rpc.Context{Slot: 123456789},
		Value:   responseValue,
	}
	return response, err
}
