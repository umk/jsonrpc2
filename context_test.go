package jsonrpc2

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Person struct {
	Name string `json:"name" validate:"required"`
	Age  int    `json:"age" validate:"gte=0"`
}

func TestRPCParseError(t *testing.T) {
	baseErr := errors.New("test error")
	parseErr := rpcParseError{err: baseErr}

	// Test Error method
	assert.Equal(t, "failed to parse RPC request: test error", parseErr.Error())

	// Test Unwrap method
	assert.True(t, errors.Is(parseErr, baseErr), "Expected parseErr to wrap baseErr")

	// Test RPCError method
	rpcErr := parseErr.RPCError()
	assert.Equal(t, -32602, rpcErr.Code)
	assert.Equal(t, baseErr.Error(), rpcErr.Message)
}

func TestGetRequestBody_WithParams(t *testing.T) {
	// Create sample JSON params
	ctx := &rpcContext{req: rpcRequest{
		Params: json.RawMessage(`{"name":"John","age":30}`),
	}}
	var data Person

	err := ctx.GetRequestBody(&data)
	require.NoError(t, err)

	assert.Equal(t, "John", data.Name)
	assert.Equal(t, 30, data.Age)
}

func TestGetRequestBody_InvalidJSON(t *testing.T) {
	// Create invalid JSON params
	ctx := &rpcContext{req: rpcRequest{
		Params: json.RawMessage(`{"name":"John","age":invalid}`),
	}}
	var data Person

	err := ctx.GetRequestBody(&data)
	assert.Error(t, err)

	// Verify it's an rpcParseError
	rpcParseErr, ok := err.(rpcParseError)
	assert.True(t, ok, "Expected rpcParseError, got %T", err)

	// Check if RPCError method works as expected
	rpcErr := rpcParseErr.RPCError()
	assert.Equal(t, -32602, rpcErr.Code)
}

func TestGetRequestBody_NoParams(t *testing.T) {
	// Request with nil params
	ctx := &rpcContext{req: rpcRequest{
		Params: nil,
	}}
	var data Person

	err := ctx.GetRequestBody(&data)
	assert.Error(t, err)

	// Verify it's an rpcParseError (due to validation)
	_, ok := err.(rpcParseError)
	assert.True(t, ok, "Expected rpcParseError, got %T", err)
}

func TestGetRequestBody_ValidationFailed(t *testing.T) {
	// Create params with invalid value (required field missing)
	ctx := &rpcContext{req: rpcRequest{
		Params: json.RawMessage(`{"age":30}`),
	}}
	var data Person

	err := ctx.GetRequestBody(&data)
	assert.Error(t, err)

	// Verify it's an rpcParseError (from validation)
	rpcParseErr, ok := err.(rpcParseError)
	assert.True(t, ok, "Expected rpcParseError, got %T", err)

	// Check error message contains validation info
	rpcErr := rpcParseErr.RPCError()
	assert.Contains(t, rpcErr.Message, "required")
}

func TestGetRequestBody_NegativeAgeValidationFailed(t *testing.T) {
	// Create params with invalid value (age negative)
	ctx := &rpcContext{req: rpcRequest{
		Params: json.RawMessage(`{"name":"John","age":-5}`),
	}}
	var data Person

	err := ctx.GetRequestBody(&data)
	assert.Error(t, err)

	// Verify it's an rpcParseError (from validation)
	rpcParseErr, ok := err.(rpcParseError)
	assert.True(t, ok, "Expected rpcParseError, got %T", err)

	// Check error message contains validation info
	rpcErr := rpcParseErr.RPCError()
	assert.Contains(t, rpcErr.Message, "gte")
}

func TestGetResponse_Valid(t *testing.T) {
	ctx := &rpcContext{}
	response := Person{
		Name: "John",
		Age:  30,
	}

	result, err := ctx.GetResponse(response)
	require.NoError(t, err)

	// Check response is returned as-is
	typedResult, ok := result.(Person)
	assert.True(t, ok, "Expected result of type person, got %T", result)
	assert.Equal(t, "John", typedResult.Name)
	assert.Equal(t, 30, typedResult.Age)
}

func TestGetResponse_Invalid(t *testing.T) {
	ctx := &rpcContext{}
	response := Person{
		// Missing required Name field
		Age: 30,
	}

	result, err := ctx.GetResponse(response)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "invalid response from server", err.Error())
}
