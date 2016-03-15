package helpers

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
	"fmt"
	"crypto/md5"
)

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// stole this from stack overflow.
func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func GetNumericSuffix(name string, sep string) (int, error) {
	namePts := strings.Split(name, sep)
	instNumStr := namePts[len(namePts)-1]
	return strconv.Atoi(instNumStr)
}

func HashInterfaceSlice(args []interface{}) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("'%s'", args))))
}
