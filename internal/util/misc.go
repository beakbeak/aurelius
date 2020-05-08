package util

import (
	"encoding/json"
	"net/http"
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

func WriteJson(
	w http.ResponseWriter,
	data interface{},
) {
	dataJson, err := json.Marshal(data)
	if err != nil {
		Debug.Printf("failed to marshal JSON: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	if _, err := w.Write(dataJson); err != nil {
		Debug.Printf("failed to write response: %v\n", err)
	}
}
