package assert

import "driftgo/utils"

func Assert(condition bool, error ...string) {
	if !condition {
		panic(utils.TTM[string](len(error) > 0, func() string { return error[0] }, "Unspecified AssertionError"))
	}
}
