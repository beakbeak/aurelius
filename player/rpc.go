package player

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sb/aurelius/util"
)

type rpcCommand struct {
	Name string `json:"cmd"`
}

// rpcHandler accepts JSON data and returns an object that will be json.Marshal()ed
type rpcHandler func(player *Player, data []byte) interface{}

var rpcHandlers map[string]rpcHandler

func init() {
	rpcHandlers = make(map[string]rpcHandler)

	rpcHandlers["next"] = rpcNext
	rpcHandlers["previous"] = rpcPrevious
	rpcHandlers["stop"] = rpcStop
	rpcHandlers["togglePause"] = rpcTogglePause
}

func (player *Player) HandleRpc(
	w http.ResponseWriter,
	req *http.Request,
) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		util.Debug.Println("failed to read RPC request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var command rpcCommand
	if err = json.Unmarshal(body, &command); err != nil {
		util.Debug.Printf("malformed RPC command: %v\n%v\n", err, string(body))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	handler, ok := rpcHandlers[command.Name]
	if !ok {
		util.Debug.Printf("unknown RPC command: %v\n", command.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	response := handler(player, body)
	responseBytes, err := json.Marshal(response)
	if err != nil {
		util.Debug.Printf("failed to marshal RPC response: %v\n", response)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	if bytesWritten, err := w.Write(responseBytes); err != nil {
		util.Debug.Printf("failed to write RPC response: %v\n", err)
	} else if bytesWritten < len(responseBytes) {
		util.Debug.Printf("wrote only %v/%v bytes of RPC response\n", bytesWritten, len(responseBytes))
	}
}

func rpcNext(player *Player, data []byte) interface{} {
	player.Next()
	return struct{}{}
}

func rpcPrevious(player *Player, data []byte) interface{} {
	player.Previous()
	return struct{}{}
}

func rpcStop(player *Player, data []byte) interface{} {
	player.Stop()
	return struct{}{}
}

func rpcTogglePause(player *Player, data []byte) interface{} {
	player.TogglePause()
	return struct{}{}
}
