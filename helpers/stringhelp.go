package helpers

import (
	"strconv"
	"strings"
)

func GetNumericSuffix(name string) (int, error) {
	namePts := strings.Split(name, "_")
	instNumStr := namePts[len(namePts)-1]
	return strconv.Atoi(instNumStr)
}
