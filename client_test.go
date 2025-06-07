package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testRequest struct {
	Input string `json:"input"`
}

type testResponse struct {
	Output string `json:"output"`
}

func TestClientCall_Success(t *testing.T) {
	inR, inW := io.Pipe()
	defer inW.Close()
	outR, outW := io.Pipe()
	defer outW.Close()

	client := NewClientFromInOut(inR, outW)
	go func() { _ = client.Read() }()

	handler := NewHandler(map[string]HandlerFunc{
		"world": func(ctx context.Context, c RPCContext) (any, error) {
			return testResponse{Output: "hello, world"}, nil
		},
	})
	server := NewServer(handler)
	go server.Run(context.Background(), outR, inW)

	var out testResponse
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := client.Call(ctx, "world", testRequest{Input: "ignored"}, &out)
	require.NoError(t, err)
	require.Equal(t, "hello, world", out.Output)
}

func TestClientCall_ErrorResponse(t *testing.T) {
	inR, inW := io.Pipe()
	defer inW.Close()
	outR, outW := io.Pipe()
	defer outW.Close()

	client := NewClientFromInOut(inR, outW)
	go func() { _ = client.Read() }()

	handler := NewHandler(map[string]HandlerFunc{
		"foo": func(ctx context.Context, c RPCContext) (any, error) {
			return nil, Error{Code: 123, Message: "some error"}
		},
	})
	server := NewServer(handler)
	go server.Run(context.Background(), outR, inW)

	var out testResponse
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := client.Call(ctx, "foo", nil, &out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "RPC error 123: some error")
}

func TestClientCall_IgnoreInvalidThenSuccess(t *testing.T) {
	inR, inW := io.Pipe()
	defer inW.Close()
	outR, outW := io.Pipe()
	defer outW.Close()

	client := NewClientFromInOut(inR, outW)
	go func() { _ = client.Read() }()

	handler := NewHandler(map[string]HandlerFunc{
		"ignored": func(ctx context.Context, c RPCContext) (any, error) {
			return testResponse{Output: "ok"}, nil
		},
	})
	server := NewServer(handler)
	go server.Run(context.Background(), outR, inW)

	// send invalid JSON-RPC message directly to client before valid response
	fmt.Fprintln(inW, `{"foo":"bar"}`)

	var out testResponse
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := client.Call(ctx, "ignored", nil, &out)
	require.NoError(t, err)
	require.Equal(t, "ok", out.Output)
}

func TestClientRead_BadJSON(t *testing.T) {
	r, w := io.Pipe()
	client := NewClientFromInOut(r, io.Discard)

	// send malformed JSON to client
	go func() {
		fmt.Fprintln(w, `{"bad"`)
		w.Close()
	}()

	err := client.Read()
	require.Error(t, err)
	var syntaxErr *json.SyntaxError
	require.ErrorAs(t, err, &syntaxErr)
}
