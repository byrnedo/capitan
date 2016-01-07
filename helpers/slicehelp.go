package helpers

import "fmt"

func ToStringSlice(data []interface{}) (out []string) {
	out = make([]string, len(data))
	for i, item := range data {
		out[i] = fmt.Sprintf("%s", item)
	}
	return
}

func ToInterfaceSlice(data []string) (out []interface{}) {
	out = make([]interface{}, len(data))
	for i, item := range data {
		out[i] = item
	}
	return
}
