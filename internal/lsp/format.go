package lsp

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"

	"github.com/gnolang/gnopls/internal/tools"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *server) Formatting(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentFormattingParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return sendParseError(ctx, reply, err)
	}

	uri := params.TextDocument.URI

	formatted, err := tools.Format(uri.Filename())
	if err != nil {
		return replyErr(ctx, reply, err)
	}

	slog.Info("format " + string(params.TextDocument.URI.Filename()))
	return reply(ctx, []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End: protocol.Position{
					Line:      math.MaxInt32,
					Character: math.MaxInt32,
				},
			},
			NewText: string(formatted),
		},
	}, nil)
}
