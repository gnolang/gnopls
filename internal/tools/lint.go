package tools

import (
	"context"

	"github.com/gnolang/tlin/lint"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func Lint(ctx context.Context, conn jsonrpc2.Conn, text string, uri protocol.DocumentURI) ([]protocol.Diagnostic, error) {
	parsedText := []byte(text)

	engine, err := lint.New("", parsedText)
	if err != nil {
		return nil, err
	}
	issues, err := lint.ProcessSource(engine, parsedText)
	if err != nil {
		return nil, err
	}

	// send the diagnostics
	diagnostics := make([]protocol.Diagnostic, len(issues))
	for i, issue := range issues {
		diagnostics[i] = protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(issue.Start.Line - 1),
					Character: uint32(issue.Start.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(issue.End.Line - 1),
					Character: uint32(issue.End.Column - 1),
				},
			},
			Severity: protocol.DiagnosticSeverityError,
			Code:     issue.Rule,
			Message:  issue.Message,
			Source:   "gnopls",
		}
	}
	return diagnostics, nil
}
