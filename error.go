package jsonrpc2

import "errors"

// Error represents an application-level JSON-RPC error with a custom code and data.
type Error struct {
	Code    int    // JSON-RPC error code, typically in -32000 to -32099 or application-defined
	Message string // human-readable error message
	Data    any    // additional error data to include
}

// Error implements the error interface.
func (e Error) Error() string {
	return e.Message
}

func getRPCError(err error) (Error, bool) {
	for err != nil {
		if rpcErr, ok := err.(Error); ok {
			return rpcErr, true
		}
		if rpcErr, ok := err.(interface{ RPCError() Error }); ok {
			return rpcErr.RPCError(), true
		}
		err = errors.Unwrap(err)
	}

	return Error{}, false
}

func getRPCErrorOrDefault(err error) Error {
	if rpcErr, ok := getRPCError(err); ok {
		return rpcErr
	}
	return Error{Code: -32603, Message: "Internal error"}
}
