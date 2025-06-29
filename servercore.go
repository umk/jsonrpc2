package jsonrpc2

import (
	"context"
	"encoding/json"
)

type serverCore struct {
	writer  *messageWriter
	handler *Handler
}

func newServerCore(writer *messageWriter, handler *Handler) *serverCore {
	return &serverCore{
		writer:  writer,
		handler: handler,
	}
}

func (s *serverCore) request(ctx context.Context, message message[rpcRequest]) error {
	var req rpcRequest
	if err := message.Get(&req); err != nil {
		resp := rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		}
		if b, err := json.Marshal(resp); err != nil {
			// Do nothing
		} else if err := s.writer.Write(b); err != nil {
			// Do nothing
		}
		return getDispatchError(err)
	}

	resp := s.handler.Handle(ctx, req)

	if resp.ID == nil {
		return nil
	}

	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return s.writer.Write(b)
}
