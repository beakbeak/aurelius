package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type RpcCommand struct {
	Name string `json:"cmd"`
}

// RpcHandler accepts JSON data and returns an object that will be json.Marshal()ed
type RpcHandler func(data []byte) interface{}

var rpcHandlers map[string]RpcHandler

func init() {
	rpcHandlers = make(map[string]RpcHandler)

	rpcHandlers["next"] = RpcNext
	rpcHandlers["previous"] = RpcPrevious
	rpcHandlers["stop"] = RpcStop
	rpcHandlers["togglePause"] = RpcTogglePause
}

func dispatchRpc(
	w http.ResponseWriter,
	req *http.Request,
) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		debug.Println("failed to read RPC request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var command RpcCommand
	if err = json.Unmarshal(body, &command); err != nil {
		debug.Printf("malformed RPC command: %v\n%v\n", err, string(body))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	handler, ok := rpcHandlers[command.Name]
	if !ok {
		debug.Printf("unknown RPC command: %v\n", command.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	response := handler(body)
	responseBytes, err := json.Marshal(response)
	if err != nil {
		debug.Printf("failed to marshal RPC response: %v\n", response)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	if bytesWritten, err := w.Write(responseBytes); err != nil {
		debug.Printf("failed to write RPC response: %v\n", err)
	} else if bytesWritten < len(responseBytes) {
		debug.Printf("wrote only %v/%v bytes of RPC response\n", bytesWritten, len(responseBytes))
	}
}

func RpcNext(data []byte) interface{} {
	player.Next()
	return struct{}{}
}

func RpcPrevious(data []byte) interface{} {
	player.Previous()
	return struct{}{}
}

func RpcStop(data []byte) interface{} {
	player.Stop()
	return struct{}{}
}

func RpcTogglePause(data []byte) interface{} {
	player.TogglePause()
	return struct{}{}
}
