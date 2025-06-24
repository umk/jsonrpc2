package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
)

type dispatcher interface {
	dispatch(ctx context.Context, buf []byte) error
}

func newDispatcher(client *clientCore, server *serverCore) dispatcher {
	switch {
	case client != nil && server != nil:
		return &duplexDispatcher{client: client, server: server}
	case client != nil:
		return &clientDispatcher{client: client}
	case server != nil:
		return &serverDispatcher{server: server}
	}

	return nil
}

type serverDispatcher struct {
	server *serverCore
}

func (d *serverDispatcher) dispatch(ctx context.Context, buf []byte) error {
	return d.server.request(ctx, messageBuf[rpcRequest](buf))
}

type clientDispatcher struct {
	client *clientCore
}

func (d *clientDispatcher) dispatch(ctx context.Context, buf []byte) error {
	return d.client.requestResolve(messageBuf[rpcResponse](buf))
}

type duplexDispatcher struct {
	client *clientCore
	server *serverCore
}

func (d *duplexDispatcher) dispatch(ctx context.Context, buf []byte) error {
	var message rpcMessage
	if err := json.Unmarshal(buf, &message); err != nil {
		return getDispatchError(err)
	}

	if message.Method != "" {
		return d.server.request(ctx, messageVal[rpcRequest]{
			message: message.asRequest(),
		})
	} else {
		return d.client.requestResolve(messageVal[rpcResponse]{
			message: message.asResponse(),
		})
	}
}

func getDispatchError(parseErr error) error {
	var syntaxErr *json.SyntaxError
	if errors.As(parseErr, &syntaxErr) {
		// A syntax error is a protocol violation that may result in an
		// undefined behavior, so the error is returned with further
		// termination of the host.
		return parseErr
	}
	// Continue processing messages without terminating the host.
	return nil
}
