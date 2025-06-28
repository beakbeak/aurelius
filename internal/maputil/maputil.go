// Package maputil provides utilities for working with string maps.
package maputil

import "strings"

// FilterKeys returns a new map containing only the key-value pairs where the filter function returns true.
// The filter function can also transform the key.
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

// LowerCaseKeys returns a new map with all keys converted to lowercase.
func LowerCaseKeys(data map[string]string) map[string]string {
	return FilterKeys(data, func(s string) (string, bool) {
		return strings.ToLower(s), true
	})
}
