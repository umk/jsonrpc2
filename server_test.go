package jsonrpc2

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReader simulates various reader behaviors for testing
type mockReader struct {
	data    []byte
	pos     int
	err     error
	delay   time.Duration
	readErr bool
}

func (r *mockReader) Read(p []byte) (int, error) {
	if r.readErr {
		return 0, r.err
	}

	if r.delay > 0 {
		time.Sleep(r.delay)
	}

	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) && r.err != nil {
		return n, r.err
	}
	return n, nil
}

func TestNewServer(t *testing.T) {
	t.Run("WithDefaultOptions", func(t *testing.T) {
		handler := NewHandler(nil)
		server := NewServer(handler)

		require.NotNil(t, server, "Expected non-nil server")
		assert.Equal(t, handler, server.handler, "Handler not correctly set")
		assert.NotNil(t, server.bufferPool, "Buffer pool not initialized")
	})

	t.Run("WithCustomOptions", func(t *testing.T) {
		handler := NewHandler(nil)
		customSize := 8192
		server := NewServer(handler, WithRequestSize(customSize))

		require.NotNil(t, server, "Expected non-nil server")
		assert.Equal(t, handler, server.handler, "Handler not correctly set")
	})
}

func TestServer_Run(t *testing.T) {
	t.Run("ProcessSingleRequest", func(t *testing.T) {
		requestJSON := `{"jsonrpc":"2.0","method":"test","id":1}` + "\n"

		// Create a handler that returns a predefined response
		handler := NewHandler(map[string]HandlerFunc{
			"test": func(ctx context.Context, c RPCContext) (any, error) {
				return "success", nil
			},
		})

		server := NewServer(handler)

		input := strings.NewReader(requestJSON)
		var output bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := server.Run(ctx, input, &output)
		// After timeout, we should get a context canceled error or nil
		if err != nil {
			assert.Equal(t, context.DeadlineExceeded, err, "Server.Run() unexpected error")
		}

		// Check that we got a valid response
		outputStr := output.String()
		var resp rpcResponse
		err = json.Unmarshal([]byte(strings.TrimSuffix(outputStr, "\n")), &resp)
		assert.NoError(t, err, "Failed to parse response")

		assert.Nil(t, resp.Error, "Got error response")
		assert.Equal(t, "success", resp.Result, "Expected result 'success'")
	})

	t.Run("ProcessMultipleRequests", func(t *testing.T) {
		request1 := `{"jsonrpc":"2.0","method":"method1","id":1}` + "\n"
		request2 := `{"jsonrpc":"2.0","method":"method2","id":2}` + "\n"

		// Create a handler with multiple methods
		handler := NewHandler(map[string]HandlerFunc{
			"method1": func(ctx context.Context, c RPCContext) (any, error) {
				return "result1", nil
			},
			"method2": func(ctx context.Context, c RPCContext) (any, error) {
				return "result2", nil
			},
		})

		server := NewServer(handler)

		input := strings.NewReader(request1 + request2)
		var output bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := server.Run(ctx, input, &output)
		// After timeout, we should get a context canceled error or nil
		if err != nil {
			assert.Equal(t, context.DeadlineExceeded, err, "Server.Run() unexpected error")
		}

		// Check we got both responses (order may vary due to concurrency)
		outputStr := output.String()
		assert.Contains(t, outputStr, `"result":"result1"`, "Expected result1 in output")
		assert.Contains(t, outputStr, `"result":"result2"`, "Expected result2 in output")
		assert.Contains(t, outputStr, `"id":1`, "Expected id:1 in output")
		assert.Contains(t, outputStr, `"id":2`, "Expected id:2 in output")
	})

	t.Run("HandleReaderError", func(t *testing.T) {
		mockReader := &mockReader{
			err:     io.ErrUnexpectedEOF,
			readErr: true,
		}

		handler := NewHandler(nil)
		server := NewServer(handler)
		var output bytes.Buffer

		err := server.Run(context.Background(), mockReader, &output)
		assert.Equal(t, io.ErrUnexpectedEOF, err, "Expected io.ErrUnexpectedEOF")
	})

	t.Run("HandleEOF", func(t *testing.T) {
		mockReader := &mockReader{
			data: []byte{}, // Empty data will trigger EOF
		}

		handler := NewHandler(nil)
		server := NewServer(handler)
		var output bytes.Buffer

		err := server.Run(context.Background(), mockReader, &output)
		assert.NoError(t, err, "Expected no error on EOF")
	})

	t.Run("HandleNullResponse", func(t *testing.T) {
		requestJSON := `{"jsonrpc":"2.0","method":"notification"}` + "\n" // No ID = notification

		// Create a handler that processes notifications
		handler := NewHandler(map[string]HandlerFunc{
			"notification": func(ctx context.Context, c RPCContext) (any, error) {
				// For a notification, return doesn't matter
				return nil, nil
			},
		})

		server := NewServer(handler)

		input := strings.NewReader(requestJSON)
		var output bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := server.Run(ctx, input, &output)
		// After timeout, we should get a context canceled error or nil
		if err != nil {
			assert.Equal(t, context.DeadlineExceeded, err, "Server.Run() unexpected error")
		}

		// Should have no output for a notification request
		assert.Equal(t, 0, output.Len(), "Expected empty output for notification")
	})

	t.Run("HandleHandlerError", func(t *testing.T) {
		requestJSON := `{"jsonrpc":"2.0","method":"errorMethod","id":1}` + "\n"

		// Create a handler that returns an error
		handler := NewHandler(map[string]HandlerFunc{
			"errorMethod": func(ctx context.Context, c RPCContext) (any, error) {
				return nil, errors.New("handler error")
			},
		})

		server := NewServer(handler)

		input := strings.NewReader(requestJSON)
		var output bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := server.Run(ctx, input, &output)
		// After timeout, we should get a context canceled error or nil
		if err != nil {
			assert.Equal(t, context.DeadlineExceeded, err, "Server.Run() unexpected error")
		}

		// Should get an error response
		outputStr := output.String()
		assert.Contains(t, outputStr, `"error"`, "Expected error in response")
	})
}

func TestReadInput(t *testing.T) {
	t.Run("SingleLine", func(t *testing.T) {
		input := "test input\n"
		reader := bufio.NewReader(strings.NewReader(input))

		data := make([]byte, 0, 64)
		err := readInput(reader, &data)

		assert.NoError(t, err, "readInput() error")
		assert.Equal(t, "test input", string(data), "readInput() incorrect data")
	})

	t.Run("MultiLine", func(t *testing.T) {
		input := "line 1\nline 2\n"
		reader := bufio.NewReader(strings.NewReader(input))

		// First call should read "line 1"
		data := make([]byte, 0, 64)
		err := readInput(reader, &data)

		assert.NoError(t, err, "readInput() first call error")
		assert.Equal(t, "line 1", string(data), "readInput() first line incorrect")

		// Second call should read "line 2"
		data = data[:0]
		err = readInput(reader, &data)

		assert.NoError(t, err, "readInput() second call error")
		assert.Equal(t, "line 2", string(data), "readInput() second line incorrect")
	})

	t.Run("LongLine", func(t *testing.T) {
		// Generate a long line that will exceed the default buffer
		longLine := strings.Repeat("a", 8192) + "\n"
		reader := bufio.NewReader(strings.NewReader(longLine))

		data := make([]byte, 0, 64)
		err := readInput(reader, &data)

		assert.NoError(t, err, "readInput() error")
		assert.Equal(t, 8192, len(data), "readInput() incorrect length")
	})

	t.Run("Error", func(t *testing.T) {
		mockReader := &mockReader{
			err:     io.ErrUnexpectedEOF,
			readErr: true,
		}
		reader := bufio.NewReader(mockReader)

		data := make([]byte, 0, 64)
		err := readInput(reader, &data)

		assert.Equal(t, io.ErrUnexpectedEOF, err, "Expected io.ErrUnexpectedEOF")
	})

	t.Run("NoNewline", func(t *testing.T) {
		// Input without a trailing newline
		input := "input without newline"
		reader := bufio.NewReader(strings.NewReader(input))

		data := make([]byte, 0, 64)
		err := readInput(reader, &data)

		assert.NoError(t, err, "readInput() error")
		assert.Equal(t, "input without newline", string(data), "readInput() incorrect data")
	})
}
