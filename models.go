package jsonrpc2

import "encoding/json"

// rpcRequest represents a JSON-RPC 2.0 rpcRequest object.
type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  *json.RawMessage `json:"params,omitempty"`
	Id      *json.RawMessage `json:"id,omitempty"`
}

// rpcResponse represents a JSON-RPC 2.0 rpcResponse object.
type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
	Id      *json.RawMessage `json:"id,omitempty"`
}

// rpcError represents a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
