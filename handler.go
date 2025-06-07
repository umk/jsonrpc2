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

func (h *Handler) Handle(ctx context.Context, data []byte) ([]byte, error) {
	var req rpcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		resp := rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		}
		return json.Marshal(resp)
	}

	resp := h.getResponse(ctx, &req)

	if resp.Id == nil {
		return nil, nil
	}

	return json.Marshal(resp)
}

func (h *Handler) getResponse(ctx context.Context, req *rpcRequest) rpcResponse {
	if req.JSONRPC != "2.0" || req.Method == "" {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32600, Message: "Invalid request"},
			Id:      req.Id,
		}
	}

	handler, ok := h.funcs[req.Method]
	if !ok {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32601, Message: "Method not found"},
			Id:      req.Id,
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
			Id: req.Id,
		}
	}

	return rpcResponse{
		JSONRPC: "2.0",
		Result:  result,
		Id:      req.Id,
	}
}
