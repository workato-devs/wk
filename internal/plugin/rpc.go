package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

const defaultCallTimeout = 30 * time.Second

// RPCRequest is a JSON-RPC 2.0 request.
type RPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      int    `json:"id"`
}

// RPCResponse is a JSON-RPC 2.0 response.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// RPCError is the error object in a JSON-RPC 2.0 response.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// RPCClient communicates with a plugin subprocess via JSON-RPC over stdio.
type RPCClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	nextID int
	mu     sync.Mutex
}

// NewRPCClient starts a plugin subprocess and returns an RPC client for it.
func NewRPCClient(entrypoint string) (*RPCClient, error) {
	cmd := exec.Command(entrypoint)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("starting plugin process: %w", err)
	}

	return &RPCClient{
		cmd:    cmd,
		stdin:  stdinPipe,
		stdout: bufio.NewReader(stdoutPipe),
		nextID: 1,
	}, nil
}

// Call sends a JSON-RPC request and waits for the response.
func (c *RPCClient) Call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	data = append(data, '\n')

	// Write with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultCallTimeout)
	defer cancel()

	writeDone := make(chan error, 1)
	go func() {
		_, err := c.stdin.Write(data)
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			return nil, fmt.Errorf("writing request: %w", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout writing request")
	}

	// Read response with timeout
	readDone := make(chan struct {
		line []byte
		err  error
	}, 1)
	go func() {
		line, err := c.stdout.ReadBytes('\n')
		readDone <- struct {
			line []byte
			err  error
		}{line, err}
	}()

	select {
	case res := <-readDone:
		if res.err != nil {
			return nil, fmt.Errorf("reading response: %w", res.err)
		}
		var resp RPCResponse
		if err := json.Unmarshal(res.line, &resp); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// Close sends a shutdown notification and stops the plugin process.
func (c *RPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send shutdown notification (no ID = notification, but we use ID 0 for simplicity)
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "shutdown",
		ID:      0,
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	c.stdin.Write(data)

	c.stdin.Close()

	// Give the process a moment to exit gracefully
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		c.cmd.Process.Kill()
		return nil
	}
}
