package jsonrpc2

import (
	"encoding/json"
	"errors"

	"github.com/go-playground/validator/v10"
)

var Val = validator.New(validator.WithRequiredStructEnabled())

// rpcParseError represents an error that occurred while parsing an RPC request.
type rpcParseError struct {
	err error
}

func (e rpcParseError) Error() string {
	return "failed to parse RPC request: " + e.err.Error()
}

func (e rpcParseError) Unwrap() error {
	return e.err
}

func (e rpcParseError) RPCError() Error {
	return Error{Code: -32602, Message: e.err.Error()}
}

type RPCContext interface {
	GetRequestBody(v any) error
	GetResponse(v any) (any, error)
}

type rpcContext struct {
	req rpcRequest
}

func (r *rpcContext) GetRequestBody(v any) error {
	if err := json.Unmarshal(r.req.Params, v); err != nil {
		return rpcParseError{err: err}
	}

	if err := Val.Struct(v); err != nil {
		return rpcParseError{err: err}
	}

	return nil
}

func (r *rpcContext) GetResponse(v any) (any, error) {
	if err := Val.Struct(v); err != nil {
		return nil, errors.New("invalid response from server")
	}
	return v, nil
}
