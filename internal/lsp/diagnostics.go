package lsp

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *server) publishDiagnostics(ctx context.Context, conn jsonrpc2.Conn, file *GnoFile) error {
	slog.Info("Lint", "path", file.URI.Filename())

	errors, err := s.TranspileAndBuild(file)
	if err != nil {
		return err
	}

	if pkg, ok := s.cache.pkgs.Get(filepath.Dir(string(file.URI.Filename()))); ok {
		filename := filepath.Base(file.URI.Filename())
		for _, er := range pkg.TypeCheckResult.Errors() {
			// Skip errors from other files in the same package
			if !strings.HasSuffix(er.FileName, filename) {
				continue
			}
			errors = append(errors, er)
		}
	}

	diagnostics := make([]protocol.Diagnostic, 0) // Init required for JSONRPC to send an empty array
	for _, er := range errors {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range:    *posToRange(er.Line, er.Span),
			Severity: protocol.DiagnosticSeverityError,
			Source:   "gnopls",
			Message:  er.Msg,
			Code:     er.Tool,
		})
	}

	return conn.Notify(
		ctx,
		protocol.MethodTextDocumentPublishDiagnostics,
		protocol.PublishDiagnosticsParams{
			URI:         file.URI,
			Diagnostics: diagnostics,
		},
	)
}
