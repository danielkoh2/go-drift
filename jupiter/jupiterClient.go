package jupiter

import (
	"context"
	"driftgo/utils"
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ilkamo/jupiter-go/jupiter"
	"math/big"
	"slices"
)

type SwapMode string

const (
	SwapModeExactIn  jupiter.GetQuoteParamsSwapMode = "ExactIn"
	SwapModeExactOut jupiter.GetQuoteParamsSwapMode = "ExactOut"
)

type JupiterClient struct {
	url              string
	connection       *rpc.Client
	jupiterClient    *jupiter.ClientWithResponses
	lookupTableCache map[string]addresslookuptable.KeyedAddressLookupTable
}

func CreateJupiterClient(
	connection *rpc.Client,
	url string,
) *JupiterClient {
	endpoint := utils.TT(url == "", jupiter.DefaultAPIURL, url)
	client, err := jupiter.NewClientWithResponses(endpoint)
	if err != nil {
		panic(err)
	}
	return &JupiterClient{
		url:           endpoint,
		connection:    connection,
		jupiterClient: client,
	}
}

func (p *JupiterClient) GetQuote(
	inputMint solana.PublicKey,
	outputMint solana.PublicKey,
	amount *big.Int,
	maxAccounts *int,
	slippageBps *int,
	swapMode *jupiter.GetQuoteParamsSwapMode,
	onlyDirectRoutes *bool,
	excludeDexes []string,
) (*jupiter.QuoteResponse, error) {
	response, err := p.jupiterClient.GetQuoteWithResponse(context.TODO(), &jupiter.GetQuoteParams{
		InputMint:        inputMint.String(),
		OutputMint:       outputMint.String(),
		Amount:           int(amount.Int64()),
		SlippageBps:      (*jupiter.SlippageParameter)(slippageBps),
		SwapMode:         (*jupiter.GetQuoteParamsSwapMode)(swapMode),
		ExcludeDexes:     &excludeDexes,
		OnlyDirectRoutes: onlyDirectRoutes,
		MaxAccounts:      utils.TT(swapMode != nil && *swapMode == SwapModeExactOut, nil, maxAccounts),
	})
	if err != nil {
		return nil, err
	}
	return response.JSON200, nil
}

func (p *JupiterClient) GetSwap(
	quote *jupiter.QuoteResponse,
	userPublicKey solana.PublicKey,
	slippageBps *int,
) (*solana.Transaction, error) {
	if slippageBps == nil {
		slippageBps = utils.NewPtr(50)
	}
	response, err := p.jupiterClient.PostSwapWithResponse(context.TODO(), jupiter.PostSwapJSONRequestBody{
		QuoteResponse: *quote,
		UserPublicKey: userPublicKey.String(),
	})
	if err != nil {
		return nil, err
	}
	transaction := solana.Transaction{}
	err = transaction.UnmarshalBase64(response.JSON200.SwapTransaction)
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (p *JupiterClient) GetTransactionMessageAndLookupTables(
	trasaction *solana.Transaction,
) (*solana.Message, []addresslookuptable.KeyedAddressLookupTable) {
	message := trasaction.Message

	lookupTables := utils.ValuesFunc(message.AddressTableLookups, func(lookup solana.MessageAddressTableLookup) addresslookuptable.KeyedAddressLookupTable {
		return p.GetLookupTable(lookup.AccountKey)
	})
	lookupTables = slices.DeleteFunc(lookupTables, func(l addresslookuptable.KeyedAddressLookupTable) bool {
		return l.Key.IsZero()
	})
	return &message, lookupTables
}

func (p *JupiterClient) GetLookupTable(accountKey solana.PublicKey) addresslookuptable.KeyedAddressLookupTable {
	lookupTable, exists := p.lookupTableCache[accountKey.String()]
	if exists {
		return lookupTable
	}
	addressLookupTableState, err := addresslookuptable.GetAddressLookupTable(context.TODO(), p.connection, accountKey)
	if err != nil {
		return addresslookuptable.KeyedAddressLookupTable{}
	}
	return addresslookuptable.KeyedAddressLookupTable{
		Key:   accountKey,
		State: *addressLookupTableState,
	}
}

func (p *JupiterClient) GetJupiterInstructions(
	transactionMessage *solana.Message,
	inputMint solana.PublicKey,
	outputMint solana.PublicKey,
) []solana.Instruction {
	return utils.ValuesFunc(slices.DeleteFunc(transactionMessage.Instructions, func(instruction solana.CompiledInstruction) bool {
		programId := transactionMessage.AccountKeys[instruction.ProgramIDIndex]
		if programId.String() == "ComputeBudget111111111111111111111111111111" {
			return true
		}
		if programId.String() == "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA" {
			return true
		}
		if programId.String() == "11111111111111111111111111111111" {
			return true
		}
		if programId.String() == "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL" {
			mintKeyIdx := int(instruction.Accounts[3])
			mintKey := transactionMessage.AccountKeys[mintKeyIdx]
			if mintKey.Equals(inputMint) || mintKey.Equals(outputMint) {
				return true
			}
		}
		return false
	}), func(cix solana.CompiledInstruction) solana.Instruction {
		accounts, _ := cix.ResolveInstructionAccounts(transactionMessage)
		return solana.NewInstruction(transactionMessage.AccountKeys[cix.ProgramIDIndex], accounts, cix.Data)
	})
}
