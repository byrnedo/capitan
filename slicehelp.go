package main

import "fmt"

func toStringSlice(data []interface{}) (out []string) {
	out = make([]string, len(data))
	for i, item := range data {
		out[i] = fmt.Sprintf("%s", item)
	}
	return
}

func toInterfaceSlice(data []string) (out []interface{}) {
	out = make([]interface{}, len(data))
	for i, item := range data {
		out[i] = item
	}
	return
}
