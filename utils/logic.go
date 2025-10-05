package utils

import "reflect"

func TT[T any](condition bool, x T, y T) T {
	if condition {
		return x
	}
	return y
}
func TTF[T any](condition bool, x func() T, y func() T) T {
	if condition {
		return x()
	}
	return y()
}

func getValue[T any](x interface{}) T {
	if reflect.TypeOf(x).Kind() == reflect.Func {
		return x.(func() T)()
	}
	return x.(T)
}
func TTM[T any](condition bool, x interface{}, y interface{}) T {
	if condition {
		return getValue[T](x)
	}
	return getValue[T](y)
}
