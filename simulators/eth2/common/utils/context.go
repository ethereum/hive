package utils

import (
	"context"
	"time"
)

// Maximum seconds to wait on an RPC request
var rpcTimeoutSeconds = 5

func ContextTimeoutRPC(
	parent context.Context,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(
		parent,
		time.Second*time.Duration(rpcTimeoutSeconds),
	)
}
