package helpers

import (
	"strconv"
	"strings"
)

func GetNumericSuffix(name string, sep string) (int, error) {
	namePts := strings.Split(name, sep)
	instNumStr := namePts[len(namePts)-1]
	return strconv.Atoi(instNumStr)
}
