package pyth

import (
	"encoding/json"
	"errors"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

// Magic is the 32-bit number prefixed on each account.
const Magic = uint32(0xa1b2c3d4)

// V2 identifies the version 2 data format stored in an account.
const V2 = uint32(2)

// The Account type enum identifies what each Pyth account stores.
const (
	AccountTypeUnknown = uint32(iota)
	AccountTypeMapping
	AccountTypeProduct
	AccountTypePrice
)

// AccountHeader is a 16-byte header at the beginning of each account type.
type AccountHeader struct {
	Magic       uint32 // set exactly to 0xa1b2c3d4
	Version     uint32 // currently V2
	AccountType uint32 // account type following the header
	Size        uint32 // size of the account including the header
}

// Valid performs basic checks on an account.
func (h AccountHeader) Valid() bool {
	// Note: This size restriction is not enforced per protocol.
	return h.Magic == Magic && h.Version == V2 && h.Size < 65536
}

// PeekAccount determines the account type given the account's data bytes.
func PeekAccount(data []byte) uint32 {
	decoder := bin.NewBinDecoder(data)
	var header AccountHeader
	if decoder.Decode(&header) != nil || !header.Valid() {
		return AccountTypeUnknown
	}
	return header.AccountType
}

type ProductAccountHeader struct {
	AccountHeader `json:"-"`
	FirstPrice    solana.PublicKey `json:"first_price"` // first price account in list
}

// ProductAccountHeaderLen is the binary offset of the AttrsData field within RawProductAccount.
const ProductAccountHeaderLen = 48

// ProductAccount contains metadata for a single product,
// such as its symbol and its base/quote currencies.
type ProductAccount struct {
	ProductAccountHeader
	Attrs AttrsMap `json:"attrs"` // key-value string pairs of additional data
}

type RawProductAccount struct {
	ProductAccountHeader
	AttrsData [464]byte
}

// UnmarshalJSON decodes the product account contents from JSON.
func (p *ProductAccount) UnmarshalJSON(buf []byte) error {
	var inner struct {
		ProductAccountHeader
		Attrs AttrsMap `json:"attrs"` // key-value string pairs of additional data
	}
	if err := json.Unmarshal(buf, &inner); err != nil {
		return err
	}
	*p = ProductAccount{
		ProductAccountHeader: ProductAccountHeader{
			AccountHeader: AccountHeader{
				Magic:       Magic,
				Version:     V2,
				AccountType: AccountTypeProduct,
				Size:        uint32(ProductAccountHeaderLen + inner.Attrs.BinaryLen()),
			},
			FirstPrice: inner.FirstPrice,
		},
		Attrs: inner.Attrs,
	}
	return nil
}

// UnmarshalBinary decodes the product account from the on-chain format.
func (p *ProductAccount) UnmarshalBinary(buf []byte) error {
	// Start by decoding the header and raw attrs data byte array.
	decoder := bin.NewBinDecoder(buf)
	var raw RawProductAccount
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if !raw.AccountHeader.Valid() {
		return errors.New("invalid account")
	}
	if raw.AccountType != AccountTypeProduct {
		return errors.New("not a product account")
	}
	p.ProductAccountHeader = raw.ProductAccountHeader
	// Now decode AttrsData.
	// Length of attrs is determined by size value in header.
	data := raw.AttrsData[:]
	maxSize := int(p.Size) - ProductAccountHeaderLen
	if maxSize > 0 && len(data) > maxSize {
		data = data[:maxSize]
	}
	// Unmarshal attrs.
	return p.Attrs.UnmarshalBinary(data)
}

// Ema is an exponentially-weighted moving average.
type Ema struct {
	ValueComponent int64
	Numerator      int64
	Denominator    int64
}

func (p *Ema) GetValue(exponent int32) decimal.Decimal {
	return decimal.New(p.ValueComponent, exponent)
}

// PriceInfo contains a price and confidence at a specific slot.
//
// This struct can represent either a publisher's contribution or the outcome of price aggregation.
type PriceInfo struct {
	PriceComponent      int64  // current price
	ConfidenceComponent uint64 // confidence interval around the price
	Status              uint32 // status of price
	CorporateAction     uint32
	PublishSlot         uint64 // valid publishing slot
}

func (p *PriceInfo) GetPrice(exponent int32) decimal.Decimal {
	price, _, _ := p.Value(exponent)
	return price
	//return utils.MulX(
	//	utils.BN(p.PriceComponent),
	//	utils.PowX(constants.TEN, utils.BN(exponent)),
	//)
}

func (p *PriceInfo) GetConfidence(exponent int32) decimal.Decimal {
	_, confidence, _ := p.Value(exponent)
	return confidence
	//return utils.MulX(
	//	utils.BN(p.ConfidenceComponent),
	//	utils.PowX(constants.TEN, utils.BN(exponent)),
	//)
}
func (p *PriceInfo) IsZero() bool {
	return p == nil || *p == PriceInfo{}
}

// Value returns the parsed price and conf values.
//
// If ok is false, the value is invalid.
func (p *PriceInfo) Value(exponent int32) (price decimal.Decimal, conf decimal.Decimal, ok bool) {
	price = decimal.New(p.PriceComponent, exponent)
	conf = decimal.New(int64(p.ConfidenceComponent), exponent)
	ok = p.Status == PriceStatusTrading
	return
}

// HasChanged returns whether there was a change between this and another price info.
func (p *PriceInfo) HasChanged(other *PriceInfo) bool {
	return (p == nil) != (other == nil) || p.Status != other.Status || p.PublishSlot != other.PublishSlot
}

// Price status.
const (
	PriceStatusUnknown = uint32(iota)
	PriceStatusTrading
	PriceStatusHalted
	PriceStatusAuction
)

// PriceComponent contains the price and confidence contributed by a specific publisher.
type PriceComponent struct {
	Publisher solana.PublicKey // key of contributing publisher
	Aggregate PriceInfo        // price used to compute the current aggregate price
	Latest    PriceInfo        // latest price of publisher
}

// PriceAccount represents a continuously-updating price feed for a product.
type PriceAccount struct {
	AccountHeader
	PriceType                   uint32 // price or calculation type
	Exponent                    int32  // price exponent
	NumComponentPrices          uint32 // number of component prices
	NumQuoters                  uint32 // number of quoters that make up aggregate
	LastSlot                    uint64 // slot of last valid (not unknown) aggregate price
	ValidSlot                   uint64 // valid slot of aggregate price
	Twap                        Ema    // exponential moving average price
	Twac                        Ema    // exponential moving confidence interval
	Drv1Component               int64
	MinPublishers               uint8
	Drv2                        uint8
	Drv3                        int16
	Drv4                        int32
	ProductAccountKey           solana.PublicKey   // ProductAccount key
	NextPriceAccountKey         solana.PublicKey   // next PriceAccount key in linked list
	PreviousSlot                uint64             // valid slot of previous update
	PreviousPriceComponent      int64              // aggregate price of previous update
	PreviousConfidenceComponent uint64             // confidence interval of previous update
	Drv5Component               int64              // space for future derived values
	Aggregate                   PriceInfo          // aggregate price info
	PriceComponents             [32]PriceComponent // price components for each quoter
}

// UnmarshalBinary decodes the price account from the on-chain format.
func (p *PriceAccount) UnmarshalBinary(buf []byte) error {
	decoder := bin.NewBinDecoder(buf)
	if err := decoder.Decode(p); err != nil {
		return err
	}
	if !p.AccountHeader.Valid() {
		return errors.New("invalid account")
	}
	if p.AccountType != AccountTypePrice {
		return errors.New("not a price account")
	}
	return nil
}

// GetComponent returns the first price component with the given publisher key. Might return nil.
func (p *PriceAccount) GetComponent(publisher *solana.PublicKey) *PriceComponent {
	for i := range p.PriceComponents {
		if p.PriceComponents[i].Publisher == *publisher {
			return &p.PriceComponents[i]
		}
	}
	return nil
}

// MappingAccount is a piece of a singly linked-list of all products on Pyth.
type MappingAccount struct {
	AccountHeader
	Num      uint32           // number of keys
	Pad1     uint32           // reserved field
	Next     solana.PublicKey // pubkey of next mapping account
	Products [640]solana.PublicKey
}

// UnmarshalBinary decodes a mapping account from the on-chain format.
func (m *MappingAccount) UnmarshalBinary(buf []byte) error {
	decoder := bin.NewBinDecoder(buf)
	if err := decoder.Decode(m); err != nil {
		return err
	}
	if !m.AccountHeader.Valid() {
		return errors.New("invalid account")
	}
	if m.AccountType != AccountTypeMapping {
		return errors.New("not a mapping account")
	}
	return nil
}

// ProductKeys returns the slice of product keys referenced by this mapping, excluding empty entries.
func (m *MappingAccount) ProductKeys() []solana.PublicKey {
	if m.Num > uint32(len(m.Products)) {
		return nil
	}
	return m.Products[:m.Num]
}

// ProductAccountEntry is a versioned product account and its pubkey.
type ProductAccountEntry struct {
	*ProductAccount
	Pubkey solana.PublicKey `json:"pubkey"`
	Slot   uint64           `json:"slot"`
}

// PriceAccountEntry is a versioned price account and its pubkey.
type PriceAccountEntry struct {
	*PriceAccount
	Pubkey solana.PublicKey `json:"pubkey"`
	Slot   uint64           `json:"slot"`
}

// MappingAccountEntry is a versioned mapping account and its pubkey.
type MappingAccountEntry struct {
	*MappingAccount
	Pubkey solana.PublicKey `json:"pubkey"`
	Slot   uint64           `json:"slot"`
}
