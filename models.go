package jsonrpc2

import "encoding/json"

// rpcRequest represents a JSON-RPC 2.0 request object.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// rpcResponse represents a JSON-RPC 2.0 response object.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// rpcError represents a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (err *rpcError) asError() error {
	return Error{
		Code:    err.Code,
		Message: err.Message,
		Data:    err.Data,
	}
}

// rpcMessage combines the fields of both rpcRequest and rpcResponse.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func (m *rpcMessage) asRequest() rpcRequest {
	return rpcRequest{
		JSONRPC: m.JSONRPC,
		Method:  m.Method,
		Params:  m.Params,
		ID:      m.ID,
	}
}

func (m *rpcMessage) asResponse() rpcResponse {
	return rpcResponse{
		JSONRPC: m.JSONRPC,
		Result:  m.Result,
		Error:   m.Error,
		ID:      m.ID,
	}
}
