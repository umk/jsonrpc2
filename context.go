package jsonrpc2

import (
	"encoding/json"
	"errors"
)

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
	ID(v any) error
	Request(v any) error
	Response(v any) (any, error)
}

type rpcContext struct {
	req rpcRequest
}

func (r *rpcContext) ID(v any) error {
	if r.req.ID == nil {
		return nil
	}

	if err := json.Unmarshal(r.req.ID, v); err != nil {
		return rpcParseError{err: err}
	}

	return nil
}

func (r *rpcContext) Request(v any) error {
	if err := json.Unmarshal(r.req.Params, v); err != nil {
		return rpcParseError{err: err}
	}

	if err := validateIfStruct(v); err != nil {
		return rpcParseError{err: err}
	}

	return nil
}

func (r *rpcContext) Response(v any) (any, error) {
	if err := validateIfStruct(v); err != nil {
		return nil, errors.New("invalid response from server")
	}
	return v, nil
}
