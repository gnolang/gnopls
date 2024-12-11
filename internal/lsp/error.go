package lsp

import (
	"context"
	"fmt"
	"log/slog"

	"go.lsp.dev/jsonrpc2"
)

func sendParseError(ctx context.Context, reply jsonrpc2.Replier, err error) error {
	return replyErr(ctx, reply, fmt.Errorf("%w: %s", jsonrpc2.ErrParse, err))
}

func replyErr(ctx context.Context, reply jsonrpc2.Replier, err error) error {
	slog.Error(err.Error())
	return reply(ctx, nil, err)
}
