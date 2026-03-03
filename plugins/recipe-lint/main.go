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

type hookResult struct {
	Passed      bool          `json:"passed"`
	Diagnostics []interface{} `json:"diagnostics"`
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
			encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   rpcError{Code: -32700, Message: "Parse error"},
				ID:      0,
			})
			continue
		}

		switch req.Method {
		case "shutdown":
			encoder.Encode(rpcResponse{JSONRPC: "2.0", Result: "ok", ID: req.ID})
			return

		case "lint.pre_push":
			// Passthrough stub — real lint logic will replace this.
			encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Result:  hookResult{Passed: true, Diagnostics: []interface{}{}},
				ID:      req.ID,
			})

		case "lint.run":
			encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Result: map[string]string{
					"message": "Recipe linter not yet implemented",
				},
				ID: req.ID,
			})

		default:
			encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   rpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
				ID:      req.ID,
			})
		}
	}
}
