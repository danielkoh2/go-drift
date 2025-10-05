package drift

const MAX_NAME_LENGTH = 32
const DEFAULT_USER_NAME = "Main Account"
const DEFAULT_MARKET_NAME = "Default Market Name"

func EncodeName(name string) []byte {
	if len(name) > MAX_NAME_LENGTH {
		panic("Name longer than 32 characters")
	}

	var buffer []byte = make([]byte, MAX_NAME_LENGTH)
	i := 0
	for ; i < MAX_NAME_LENGTH; i++ {
		if i < len(name) {
			buffer[i] = name[i]
		} else {
			buffer[i] = (" ")[0]
		}
	}
	return buffer
}

func DecodeName(buffer []byte) string {
	return string(buffer[:])
}
