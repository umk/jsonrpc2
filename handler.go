package jsonrpc2

import (
	"context"
	"encoding/json"
)

// HandlerFunc defines the signature of JSON-RPC method handlers.
type HandlerFunc func(ctx context.Context, c RPCContext) (any, error)

type Handler struct {
	funcs map[string]HandlerFunc
}

func NewHandler(funcs map[string]HandlerFunc) *Handler {
	return &Handler{funcs: funcs}
}

func (h *Handler) Handle(ctx context.Context, req rpcRequest) rpcResponse {
	if req.JSONRPC != "2.0" || req.Method == "" {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32600, Message: "Invalid request"},
			ID:      req.ID,
		}
	}

	handler, ok := h.funcs[req.Method]
	if !ok {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32601, Message: "Method not found"},
			ID:      req.ID,
		}
	}

	rpcCtx := &rpcContext{req: req}

	result, err := handler(ctx, rpcCtx)
	if err != nil {
		rpcErr := getRPCErrorOrDefault(err)
		return rpcResponse{
			JSONRPC: "2.0",
			Error: &rpcError{
				Code:    rpcErr.Code,
				Message: rpcErr.Message,
				Data:    rpcErr.Data,
			},
			ID: req.ID,
		}
	}

	if req.ID == nil {
		return rpcResponse{}
	}

	b, err := json.Marshal(result)
	if err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32603, Message: "Internal error"},
			ID:      req.ID,
		}
	}

	return rpcResponse{
		JSONRPC: "2.0",
		Result:  json.RawMessage(b),
		ID:      req.ID,
	}
}
