package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRPCContext implements the RPCContext interface for testing
type MockRPCContext struct {
	req         *rpcRequest
	requestBody any
	response    any
	err         error
}

func (m *MockRPCContext) GetRequestBody(v any) error {
	if m.err != nil {
		return m.err
	}

	// If requestBody is set, use it to populate v
	if m.requestBody != nil {
		data, err := json.Marshal(m.requestBody)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, v)
	}

	// Otherwise, use the request params if they exist
	if m.req != nil && m.req.Params != nil {
		return json.Unmarshal(m.req.Params, v)
	}

	return nil
}

func (m *MockRPCContext) GetResponse(v any) (any, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.response = v
	return v, nil
}

func TestNewHandler(t *testing.T) {
	funcs := map[string]HandlerFunc{
		"testMethod": func(ctx context.Context, c RPCContext) (any, error) {
			return "result", nil
		},
	}

	handler := NewHandler(funcs)

	require.NotNil(t, handler, "Expected non-nil handler")
	assert.Len(t, handler.funcs, len(funcs), "Handler should have the same number of functions")
	_, ok := handler.funcs["testMethod"]
	assert.True(t, ok, "Expected 'testMethod' to be in handler functions")
}

func TestHandler_Handle_ParseError(t *testing.T) {
	// Note: This test might not be applicable anymore since we're passing
	// rpcRequest objects directly instead of parsing JSON
	t.Skip("Parse error test is no longer applicable with direct rpcRequest handling")
}

func TestHandler_Handle_InvalidRequest(t *testing.T) {
	handler := NewHandler(nil)

	testCases := []struct {
		name       string
		request    rpcRequest
		expectCode int
		expectMsg  string
	}{
		{
			name: "Missing JSONRPC Version",
			request: rpcRequest{
				Method: "test",
				Params: json.RawMessage(`{}`),
				ID:     json.RawMessage(`1`),
			},
			expectCode: -32600,
			expectMsg:  "Invalid request",
		},
		{
			name: "Wrong JSONRPC Version",
			request: rpcRequest{
				JSONRPC: "1.0",
				Method:  "test",
				Params:  json.RawMessage(`{}`),
				ID:      json.RawMessage(`1`),
			},
			expectCode: -32600,
			expectMsg:  "Invalid request",
		},
		{
			name: "Missing Method",
			request: rpcRequest{
				JSONRPC: "2.0",
				Params:  json.RawMessage(`{}`),
				ID:      json.RawMessage(`1`),
			},
			expectCode: -32600,
			expectMsg:  "Invalid request",
		},
		{
			name: "Empty Method",
			request: rpcRequest{
				JSONRPC: "2.0",
				Method:  "",
				Params:  json.RawMessage(`{}`),
				ID:      json.RawMessage(`1`),
			},
			expectCode: -32600,
			expectMsg:  "Invalid request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := handler.Handle(context.Background(), tc.request)

			require.NotNil(t, resp.Error, "Expected error response")
			assert.Equal(t, tc.expectCode, resp.Error.Code, "Wrong error code")
			assert.Equal(t, tc.expectMsg, resp.Error.Message, "Wrong error message")
		})
	}
}

func TestHandler_Handle_MethodNotFound(t *testing.T) {
	handler := NewHandler(map[string]HandlerFunc{
		"existingMethod": func(ctx context.Context, c RPCContext) (any, error) {
			return "result", nil
		},
	})

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "nonExistingMethod",
		Params:  json.RawMessage(`{}`),
		ID:      json.RawMessage(`1`),
	}

	resp := handler.Handle(context.Background(), req)

	require.NotNil(t, resp.Error, "Expected error response")
	assert.Equal(t, -32601, resp.Error.Code, "Expected method not found code")
	assert.Equal(t, "Method not found", resp.Error.Message, "Expected 'Method not found' message")
}

func TestHandler_Handle_Success(t *testing.T) {
	handler := NewHandler(map[string]HandlerFunc{
		"testMethod": func(ctx context.Context, c RPCContext) (any, error) {
			var params struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}
			if err := c.GetRequestBody(&params); err != nil {
				return nil, err
			}

			return map[string]any{
				"greeting": "Hello, " + params.Name,
				"age":      params.Age,
			}, nil
		},
	})

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "testMethod",
		Params:  json.RawMessage(`{"name": "John", "age": 30}`),
		ID:      json.RawMessage(`123`),
	}

	resp := handler.Handle(context.Background(), req)

	assert.Nil(t, resp.Error, "Expected no error in response")

	var resultMap map[string]any
	err := json.Unmarshal(resp.Result, &resultMap)
	require.NoError(t, err, "Failed to unmarshal result")

	greeting, ok := resultMap["greeting"].(string)
	assert.True(t, ok, "Expected string greeting")
	assert.Equal(t, "Hello, John", greeting, "Wrong greeting")

	age, ok := resultMap["age"].(float64) // JSON numbers are floats
	assert.True(t, ok, "Expected number age")
	assert.Equal(t, float64(30), age, "Wrong age")

	// Check ID is passed through
	var id float64
	idBytes, _ := json.Marshal(resp.ID)
	json.Unmarshal(idBytes, &id)
	assert.Equal(t, float64(123), id, "Expected id 123")
}

func TestHandler_Handle_HandlerError(t *testing.T) {
	customErr := Error{
		Code:    -32000,
		Message: "Custom application error",
		Data:    "Additional error data",
	}

	handler := NewHandler(map[string]HandlerFunc{
		"errorMethod": func(ctx context.Context, c RPCContext) (any, error) {
			return nil, customErr
		},
		"regularError": func(ctx context.Context, c RPCContext) (any, error) {
			return nil, errors.New("regular error")
		},
	})

	t.Run("RPC Error", func(t *testing.T) {
		req := rpcRequest{
			JSONRPC: "2.0",
			Method:  "errorMethod",
			ID:      json.RawMessage(`1`),
		}

		resp := handler.Handle(context.Background(), req)

		require.NotNil(t, resp.Error, "Expected error response")
		assert.Equal(t, customErr.Code, resp.Error.Code, "Wrong error code")
		assert.Equal(t, customErr.Message, resp.Error.Message, "Wrong error message")
		assert.Equal(t, customErr.Data, resp.Error.Data, "Wrong error data")
	})

	t.Run("Regular Error", func(t *testing.T) {
		req := rpcRequest{
			JSONRPC: "2.0",
			Method:  "regularError",
			ID:      json.RawMessage(`1`),
		}

		resp := handler.Handle(context.Background(), req)

		require.NotNil(t, resp.Error, "Expected error response")
		assert.Equal(t, -32603, resp.Error.Code, "Expected internal error code")
		assert.Equal(t, "Internal error", resp.Error.Message, "Expected 'Internal error' message")
	})
}

func TestHandler_Handle_Notification(t *testing.T) {
	methodCalled := false

	handler := NewHandler(map[string]HandlerFunc{
		"notificationMethod": func(ctx context.Context, c RPCContext) (any, error) {
			methodCalled = true
			return "result", nil
		},
	})

	// Notification request (no ID)
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "notificationMethod",
		Params:  json.RawMessage(`{}`),
	}

	resp := handler.Handle(context.Background(), req)

	assert.Nil(t, resp.ID, "Expected nil request ID for notification")
	assert.True(t, methodCalled, "Expected notification method to be called")
}
