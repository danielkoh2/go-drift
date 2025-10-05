package utils

import "math/rand/v2"

func ForEach[T any](array []T, f func(T, int)) {
	for idx, v := range array {
		f(v, idx)
	}
}

func RandomElement[T any](array []T) T {
	length := len(array)
	if length == 0 {
		panic("Array is empty")
	}
	idx := rand.IntN(length)
	return array[idx]
}

func ValuesFunc[T any, V any](array []T, f func(T) V, filter ...func(T) bool) []V {
	var filterFunc func(T) bool
	if len(filter) > 0 {
		filterFunc = filter[0]
	}
	var values []V
	for idx := 0; idx < len(array); idx++ {
		if filterFunc != nil && !filterFunc(array[idx]) {
			continue
		}
		values = append(values, f(array[idx]))
	}
	return values
}

func ArrayReduce[T any](array []T, callback func(T, T) T, initialValue ...T) T {
	var result T
	initialized := false
	if len(initialValue) > 0 {
		result = initialValue[0]
		initialized = true
	}
	for _, v := range array {
		if !initialized {
			result = v
			initialized = true
			continue
		}
		result = callback(result, v)
	}
	return result
}

func ArrayFlat[T any](array [][]T) []T {
	var result []T
	for _, v := range array {
		result = append(result, v...)
	}
	return result
}
