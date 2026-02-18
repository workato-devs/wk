package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      int             `json:"id"`
}

type rpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	ID      int    `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := rpcResponse{
				JSONRPC: "2.0",
				Error:   rpcError{Code: -32700, Message: "Parse error"},
				ID:      0,
			}
			encoder.Encode(resp)
			continue
		}

		switch req.Method {
		case "shutdown":
			// Acknowledge and exit
			resp := rpcResponse{JSONRPC: "2.0", Result: "ok", ID: req.ID}
			encoder.Encode(resp)
			return

		case "example.hello":
			resp := rpcResponse{
				JSONRPC: "2.0",
				Result: map[string]string{
					"message": "Hello from the example plugin!",
					"version": "0.1.0",
				},
				ID: req.ID,
			}
			encoder.Encode(resp)

		default:
			resp := rpcResponse{
				JSONRPC: "2.0",
				Error:   rpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
				ID:      req.ID,
			}
			encoder.Encode(resp)
		}
	}
}
