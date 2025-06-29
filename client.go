package jsonrpc2

import "context"

type Client interface {
	Call(ctx context.Context, method string, req any, resp any) error
	Notify(ctx context.Context, method string, req any) error
}
