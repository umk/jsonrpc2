package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCore_RequestSend_Success(t *testing.T) {
	buf := &bytes.Buffer{}
	cw := newMessageWriter(buf)
	c := newClientCore(cw)
	id := "testid"
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "foo",
		Params:  nil,
		ID:      json.RawMessage(`"testid"`),
	}
	ch, err := c.requestSend(id, req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// verify request was written correctly
	var sent rpcRequest
	err = json.Unmarshal(buf.Bytes(), &sent)
	require.NoError(t, err)
	assert.Equal(t, req, sent)

	// ensure channel registered
	_, ok := c.requests[id]
	assert.True(t, ok)
}

func TestClientCore_RequestSend_DuplicateID(t *testing.T) {
	buf := &bytes.Buffer{}
	c := newClientCore(newMessageWriter(buf))
	id := "dupe"
	// simulate existing request
	c.requests[id] = make(chan any, 1)

	_, err := c.requestSend(id, rpcRequest{
		JSONRPC: "2.0",
		Method:  "m",
		Params:  nil,
		ID:      json.RawMessage(`"dupe"`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("request %s is already sent", id))
}

func TestClientCore_RequestClose(t *testing.T) {
	c := newClientCore(newMessageWriter(&bytes.Buffer{}))
	id := "close"
	ch := make(chan any, 1)
	c.requests[id] = ch

	c.requestClose(id)

	// request should be removed
	_, ok := c.requests[id]
	assert.False(t, ok)

	// channel should be closed
	_, ok2 := <-ch
	assert.False(t, ok2)
}

func TestClientCore_Resolve_Success(t *testing.T) {
	c := newClientCore(newMessageWriter(&bytes.Buffer{}))
	id := "resp"
	ch := make(chan any, 1)
	c.requests[id] = ch

	resp := rpcResponse{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`"ok"`),
		Error:   nil,
		ID:      json.RawMessage(`"resp"`),
	}
	err := c.resolve(messageVal[rpcResponse]{message: resp})
	require.NoError(t, err)

	received := <-ch
	assert.Equal(t, resp, received)
}

func TestClientCore_Resolve_InvalidID(t *testing.T) {
	c := newClientCore(newMessageWriter(&bytes.Buffer{}))
	// no registered requests
	resp := rpcResponse{
		JSONRPC: "2.0",
		Result:  nil,
		Error:   nil,
		ID:      json.RawMessage(`123`),
	}
	err := c.resolve(messageVal[rpcResponse]{message: resp})
	require.NoError(t, err)
}

func TestClientCore_Resolve_GetError(t *testing.T) {
	c := newClientCore(newMessageWriter(&bytes.Buffer{}))
	// invalid JSON should produce a syntax error
	bad := messageBuf[rpcResponse]([]byte("bad"))
	err := c.resolve(bad)
	require.Error(t, err)
	var synErr *json.SyntaxError
	assert.ErrorAs(t, err, &synErr)
}

func TestClientCore_Call_RequestMarshalError(t *testing.T) {
	// struct with unmarshalable field (channel) to force JSON marshal error
	type badReq struct{ C chan int }
	c := newClientCore(newMessageWriter(&bytes.Buffer{}))
	err := c.Call(context.Background(), "m", badReq{C: make(chan int)}, new(any))
	require.Error(t, err)
}

func TestClientCore_Call_ContextCanceled(t *testing.T) {
	buf := &bytes.Buffer{}
	c := newClientCore(newMessageWriter(buf))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out any
	err := c.Call(ctx, "m", nil, &out)
	require.Error(t, err)
	assert.Equal(t, context.Cause(ctx), err)
}
