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
		return json.Unmarshal(*m.req.Params, v)
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
	handler := NewHandler(nil)

	// Invalid JSON
	data := []byte(`{"jsonrpc": "2.0", "method": "test", "params": {,}}`)

	resp, err := handler.Handle(context.Background(), data)
	require.NoError(t, err, "Expected no error")

	var result rpcResponse
	err = json.Unmarshal(resp, &result)
	require.NoError(t, err, "Failed to unmarshal response")

	require.NotNil(t, result.Error, "Expected error response")
	assert.Equal(t, -32700, result.Error.Code, "Expected parse error code")
	assert.Equal(t, "Parse error", result.Error.Message, "Expected 'Parse error' message")
}

func TestHandler_Handle_InvalidRequest(t *testing.T) {
	handler := NewHandler(nil)

	testCases := []struct {
		name        string
		requestJSON string
		expectCode  int
		expectMsg   string
	}{
		{
			name:        "Missing JSONRPC Version",
			requestJSON: `{"method": "test", "params": {}, "id": 1}`,
			expectCode:  -32600,
			expectMsg:   "Invalid request",
		},
		{
			name:        "Wrong JSONRPC Version",
			requestJSON: `{"jsonrpc": "1.0", "method": "test", "params": {}, "id": 1}`,
			expectCode:  -32600,
			expectMsg:   "Invalid request",
		},
		{
			name:        "Missing Method",
			requestJSON: `{"jsonrpc": "2.0", "params": {}, "id": 1}`,
			expectCode:  -32600,
			expectMsg:   "Invalid request",
		},
		{
			name:        "Empty Method",
			requestJSON: `{"jsonrpc": "2.0", "method": "", "params": {}, "id": 1}`,
			expectCode:  -32600,
			expectMsg:   "Invalid request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handler.Handle(context.Background(), []byte(tc.requestJSON))
			require.NoError(t, err, "Expected no error")

			var result rpcResponse
			err = json.Unmarshal(resp, &result)
			require.NoError(t, err, "Failed to unmarshal response")

			require.NotNil(t, result.Error, "Expected error response")
			assert.Equal(t, tc.expectCode, result.Error.Code, "Wrong error code")
			assert.Equal(t, tc.expectMsg, result.Error.Message, "Wrong error message")
		})
	}
}

func TestHandler_Handle_MethodNotFound(t *testing.T) {
	handler := NewHandler(map[string]HandlerFunc{
		"existingMethod": func(ctx context.Context, c RPCContext) (any, error) {
			return "result", nil
		},
	})

	data := []byte(`{"jsonrpc": "2.0", "method": "nonExistingMethod", "params": {}, "id": 1}`)

	resp, err := handler.Handle(context.Background(), data)
	require.NoError(t, err, "Expected no error")

	var result rpcResponse
	err = json.Unmarshal(resp, &result)
	require.NoError(t, err, "Failed to unmarshal response")

	require.NotNil(t, result.Error, "Expected error response")
	assert.Equal(t, -32601, result.Error.Code, "Expected method not found code")
	assert.Equal(t, "Method not found", result.Error.Message, "Expected 'Method not found' message")
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

	data := []byte(`{
		"jsonrpc": "2.0", 
		"method": "testMethod", 
		"params": {"name": "John", "age": 30}, 
		"id": 123
	}`)

	resp, err := handler.Handle(context.Background(), data)
	require.NoError(t, err, "Expected no error")

	var result rpcResponse
	err = json.Unmarshal(resp, &result)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.Nil(t, result.Error, "Expected no error in response")

	resultMap, ok := result.Result.(map[string]any)
	require.True(t, ok, "Expected map result")

	greeting, ok := resultMap["greeting"].(string)
	assert.True(t, ok, "Expected string greeting")
	assert.Equal(t, "Hello, John", greeting, "Wrong greeting")

	age, ok := resultMap["age"].(float64) // JSON numbers are floats
	assert.True(t, ok, "Expected number age")
	assert.Equal(t, float64(30), age, "Wrong age")

	// Check ID is passed through
	var id float64
	idBytes, _ := json.Marshal(result.Id)
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
		data := []byte(`{"jsonrpc": "2.0", "method": "errorMethod", "id": 1}`)

		resp, err := handler.Handle(context.Background(), data)
		require.NoError(t, err, "Expected no error")

		var result rpcResponse
		err = json.Unmarshal(resp, &result)
		require.NoError(t, err, "Failed to unmarshal response")

		require.NotNil(t, result.Error, "Expected error response")
		assert.Equal(t, customErr.Code, result.Error.Code, "Wrong error code")
		assert.Equal(t, customErr.Message, result.Error.Message, "Wrong error message")
		assert.Equal(t, customErr.Data, result.Error.Data, "Wrong error data")
	})

	t.Run("Regular Error", func(t *testing.T) {
		data := []byte(`{"jsonrpc": "2.0", "method": "regularError", "id": 1}`)

		resp, err := handler.Handle(context.Background(), data)
		require.NoError(t, err, "Expected no error")

		var result rpcResponse
		err = json.Unmarshal(resp, &result)
		require.NoError(t, err, "Failed to unmarshal response")

		require.NotNil(t, result.Error, "Expected error response")
		assert.Equal(t, -32603, result.Error.Code, "Expected internal error code")
		assert.Equal(t, "Internal error", result.Error.Message, "Expected 'Internal error' message")
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
	data := []byte(`{"jsonrpc": "2.0", "method": "notificationMethod", "params": {}}`)

	resp, err := handler.Handle(context.Background(), data)
	assert.NoError(t, err, "Expected no error")
	assert.Nil(t, resp, "Expected nil response for notification")
	assert.True(t, methodCalled, "Expected notification method to be called")
}
