package util

import (
	"strings"
)

func FilterKeys(
	data map[string]string,
	filter func(string) (string, bool),
) map[string]string {
	out := make(map[string]string, len(data))
	for key, value := range data {
		if outKey, ok := filter(key); ok {
			out[outKey] = value
		}
	}
	return out
}

func LowerCaseKeys(data map[string]string) map[string]string {
	return FilterKeys(data, func(s string) (string, bool) {
		return strings.ToLower(s), true
	})
}
