package geyser

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"time"
)

func GenerateIdentity() string {
	identity := fmt.Sprintf("%d-%d", time.Now().UnixMicro(), rand.Int63())
	h := md5.New()
	h.Write([]byte(identity))
	return fmt.Sprintf("%x", h.Sum(nil))
}
