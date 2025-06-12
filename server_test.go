package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerCore_Request_ParseError(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newMessageWriter(buf)
	handler := NewHandler(nil)
	sc := newServerCore(writer, handler)
	// invalid JSON triggers json.SyntaxError
	msg := messageBuf[rpcRequest]([]byte("not json"))
	err := sc.request(context.Background(), msg)
	// Syntax errors are returned
	require.Error(t, err)
	assert.IsType(t, &json.SyntaxError{}, err)

	// Expect error response
	out := strings.TrimSpace(buf.String())
	require.NotEmpty(t, out)

	var resp rpcResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32700, resp.Error.Code)
}

func TestServerCore_Request_Success(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newMessageWriter(buf)
	// Handler returns a simple string result
	h := NewHandler(map[string]HandlerFunc{
		"ping": func(ctx context.Context, c RPCContext) (any, error) {
			return "pong", nil
		},
	})
	sc := newServerCore(writer, h)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "ping",
		Params:  json.RawMessage(`null`),
		ID:      json.RawMessage(`42`),
	}
	err := sc.request(context.Background(), messageVal[rpcRequest]{message: req})
	require.NoError(t, err)

	out := strings.TrimSpace(buf.String())
	expected := `{"jsonrpc":"2.0","result":"pong","id":42}`
	assert.Equal(t, expected, out)
}

func TestServerCore_Request_Notification(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newMessageWriter(buf)
	// Handler should be invoked but no response written
	called := false
	h := NewHandler(map[string]HandlerFunc{
		"notify": func(ctx context.Context, c RPCContext) (any, error) {
			called = true
			return "ok", nil
		},
	})
	sc := newServerCore(writer, h)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "notify",
		Params:  json.RawMessage(`null`),
		ID:      nil,
	}
	err := sc.request(context.Background(), messageVal[rpcRequest]{message: req})
	require.NoError(t, err)
	assert.True(t, called, "notification handler should be called")
	assert.Empty(t, buf.String(), "no response should be written for notifications")
}

func TestServerCore_Request_HandlerError(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newMessageWriter(buf)
	// Handler returns a general error
	h := NewHandler(map[string]HandlerFunc{
		"fail": func(ctx context.Context, c RPCContext) (any, error) {
			return nil, errors.New("handler failure")
		},
	})
	sc := newServerCore(writer, h)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "fail",
		Params:  json.RawMessage(`null`),
		ID:      json.RawMessage(`7`),
	}
	err := sc.request(context.Background(), messageVal[rpcRequest]{message: req})
	require.NoError(t, err)

	out := strings.TrimSpace(buf.String())

	var resp rpcResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32603, resp.Error.Code)
	assert.Equal(t, "Internal error", resp.Error.Message)
}

func TestServerCore_Request_CustomRPCError(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newMessageWriter(buf)
	// Handler returns a custom RPC Error
	customErr := Error{Code: -32050, Message: "Custom error", Data: "extra"}
	h := NewHandler(map[string]HandlerFunc{
		"custom": func(ctx context.Context, c RPCContext) (any, error) {
			return nil, customErr
		},
	})
	sc := newServerCore(writer, h)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "custom",
		Params:  json.RawMessage(`null`),
		ID:      json.RawMessage(`100`),
	}
	err := sc.request(context.Background(), messageVal[rpcRequest]{message: req})
	require.NoError(t, err)

	out := strings.TrimSpace(buf.String())

	var resp rpcResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.NotNil(t, resp.Error)
	assert.Equal(t, customErr.Code, resp.Error.Code)
	assert.Equal(t, customErr.Message, resp.Error.Message)
	assert.Equal(t, customErr.Data, resp.Error.Data)
}
