package types

//import (
//	"context"
//	go_drift "driftgo"
//	"driftgo/anchor"
//	"driftgo/connection"
//	"driftgo/lib/drift"
//	"github.com/gagliardetto/solana-go"
//	"github.com/gagliardetto/solana-go/rpc"
//)
//
//type PythLazerClient struct {
//	IOracleClient,
//	connection *rpc.Client
//}
//
//const DriftProgramId = "dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH"
//
//func CreatePythLazerClient(connection *rpc.Client) *PythLazerClient {
//	return &PythLazerClient{connection: connection}
//}
//
//func (p *PythLazerClient) GetOraclePriceData(pricePublicKey solana.PublicKey) *OraclePriceData {
//	out, err := p.connection.GetAccountInfo(context.TODO(), pricePublicKey)
//	if err != nil {
//		return nil
//	}
//	return p.GetOraclePriceDataFromBuffer(out.GetBinary())
//}
//
//func (p *PythLazerClient) GetOraclePriceDataFromBuffer(buffer []byte) *OraclePriceData {
//	priceData := p.decodeFunc("OracleSourcePyth_Lazer", buffer)
//}
//
//func (p *PythLazerClient) decodeFunc(oracleSource string, buffer []byte) *OraclePriceData {
//	keypair, err := solana.NewRandomPrivateKey()
//	if err != nil {
//		return nil
//	}
//	wallet := &go_drift.Wallet{PrivateKey: keypair}
//	provider := anchor.CreateAnchorProvider(wallet, wallet.GetPublicKey(), go_drift.ConfirmOptions{Commitment: rpc.CommitmentConfirmed}, connection.CreateManager())
//	program := anchor.CreateProgram(solana.MPK(DriftProgramId), provider)
//	return &OraclePriceData{
//		Price:                           nil,
//		Slot:                            0,
//		Confidence:                      nil,
//		HasSufficientNumberOfDataPoints: true,
//		Twap:                            nil,
//		TwapConfidence:                  nil,
//		MaxPrice:                        nil,
//	}
//}
