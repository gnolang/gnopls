package tools

import (
	"context"

	"github.com/gnolang/tlin/lint"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func Lint(ctx context.Context, conn jsonrpc2.Conn, text string, uri protocol.DocumentURI) error {
	parsedText := []byte(text)

	engine, err := lint.New("", parsedText)
	if err != nil {
		return err
	}
	issues, err := lint.ProcessSource(engine, parsedText)
	if err != nil {
		return err
	}

	// send the diagnostics
	diagnostics := make([]protocol.Diagnostic, len(issues))
	for i, issue := range issues {
		diagnostics[i] = protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(issue.Start.Line),
					Character: uint32(issue.Start.Column),
				},
				End: protocol.Position{
					Line:      uint32(issue.End.Line),
					Character: uint32(issue.End.Column),
				},
			},
			Severity: protocol.DiagnosticSeverityError,
			Code:     issue.Rule,
			Message:  issue.Message,
			Source:   "gnopls",
		}
	}
	notification := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}
	if err := conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, notification); err != nil {
		return err
	}
	return nil
}
