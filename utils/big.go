package utils

import (
	"encoding/binary"
	bin "github.com/gagliardetto/binary"
	"math"
	"math/big"
	"reflect"
)

func IntX(x *big.Int) *big.Int {
	z := big.NewInt(0)
	return z.Set(x)
}

func AddX(x *big.Int, y ...*big.Int) *big.Int {
	z := big.NewInt(0)
	z.Set(x)
	for _, v := range y {
		z = z.Add(z, v)
	}
	return z
}

func SubX(x *big.Int, y ...*big.Int) *big.Int {
	z := big.NewInt(0)
	z.Set(x)
	for _, v := range y {
		z = z.Sub(z, v)
	}
	return z
}

func MulX(x *big.Int, y ...*big.Int) *big.Int {

	z := big.NewInt(0)
	z.Set(x)
	for _, v := range y {
		z = z.Mul(z, v)
	}
	return z
}

func MulF(x *big.Float, y ...*big.Float) *big.Float {
	z := big.NewFloat(0)
	z.Set(x)
	for _, v := range y {
		z = z.Mul(z, v)
	}
	return z
}

func DivX(x *big.Int, y ...*big.Int) *big.Int {
	z := big.NewInt(0)
	z.Set(x)
	for _, v := range y {
		z = z.Div(z, v)
	}
	return z
}

func DivF(x *big.Float, y ...*big.Float) *big.Float {
	z := big.NewFloat(0)
	z.Set(x)
	for _, v := range y {
		z = z.Quo(z, v)
	}
	return z

}
func DivCeilX(x *big.Int, y *big.Int) *big.Int {
	quotient := DivX(x, y)
	remainder := x.Mod(x, y)

	if remainder.Cmp(BN(0)) != 0 {
		return AddX(quotient, BN(1))
	} else {
		return quotient
	}
}

func PowX(x, y *big.Int) *big.Int {
	z := big.NewInt(0)
	z.Set(x)
	return z.Exp(z, y, nil)
}

func ModX(x, y *big.Int) *big.Int {
	z := big.NewInt(0)
	z.Set(x)
	return z.Mod(z, y)
}

func AbsX(x *big.Int) *big.Int {
	z := big.NewInt(0)
	return z.Abs(x)
}

func NegX(x *big.Int) *big.Int {
	return x.Neg(x)
}

func Min(x *big.Int, y ...*big.Int) *big.Int {
	minValue := x
	for _, v := range y {
		if minValue.Cmp(v) > 0 {
			minValue = v
		}
	}
	return minValue
}

func Max(x *big.Int, y ...*big.Int) *big.Int {
	maxValue := x
	for _, v := range y {
		if maxValue.Cmp(v) < 0 {
			maxValue = v
		}
	}
	return maxValue
}

func Float64(x *big.Int) float64 {
	f, _ := x.Float64()
	return f
}

func BigInt64(x int64) *big.Int {
	return big.NewInt(x)
}

func BigUInt64(x uint64) *big.Int {
	return big.NewInt(0).SetUint64(x)
}

func BN[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](x T) *big.Int {
	xtype := []byte(reflect.TypeOf(x).String())
	if xtype[0] == 'u' {
		return BigUInt64(uint64(x))
	} else {
		return BigInt64(int64(x))
	}
}

func BF[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64](x T) *big.Float {
	return big.NewFloat(float64(x))
}

func SquareRootBN(n *big.Int) *big.Int {
	z := big.NewInt(0)
	return z.Sqrt(n)
	//if n.Cmp(BN(0)) < 0 {
	//	return nil
	//}
	//if n.Cmp(BN(2)) < 0 {
	//	return n
	//}
	//smallCand := SquareRootBN(n.Rsh(n, 2))
	//smallCand = smallCand.Lsh(smallCand, 1)
	//
	//largeCand := AddX(smallCand, BN(1))
	//
	//if MulX(largeCand, largeCand).Cmp(n) > 0 {
	//	return smallCand
	//} else {
	//	return largeCand
	//}
}
func Uint128(x *big.Int) (u bin.Uint128) {
	if x.Sign() < 0 {
		panic("value cannot be negative")
	} else if x.BitLen() > 128 {
		panic("value overflows Uint128")
	}
	u.Lo = x.Uint64()
	u.Hi = x.Rsh(x, 64).Uint64()
	u.Endianness = binary.LittleEndian
	return u
}

func Int128(x *big.Int) (u bin.Int128) {
	if x.BitLen() > 128 {
		panic("value overflows Uint128")
	}
	u.Lo = x.Uint64()
	u.Hi = x.Rsh(x, 64).Uint64()
	u.Endianness = binary.LittleEndian
	return u
}

func AbsN(x int64) int64 {
	return int64(math.Abs(float64(x)))
}

func SigNum(x *big.Int) *big.Int {
	if x.Sign() == -1 {
		return big.NewInt(-1)
	} else {
		return big.NewInt(1)
	}
}

func ClampBN(x *big.Int, min *big.Int, max *big.Int) *big.Int {
	return Max(min, Min(x, max))
}
