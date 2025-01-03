package lsp

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *server) getTranspileDiagnostics(file *GnoFile) ([]protocol.Diagnostic, error) {
	errors, err := s.Transpile()
	if err != nil {
		return nil, err
	}

	/*
		 NOTE(tb): not sure we really need this, bc Transpile already returns
		 all the errors we need. The ones in the cache are just duplicates or
		 potentially obsolete.

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
	*/

	diagnostics := make([]protocol.Diagnostic, 0) // Init required for JSONRPC to send an empty array
	for _, er := range errors {
		if file.URI.Filename() != er.FileName {
			// Ignore error thay does not target file
			continue
		}
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range:    *posToRange(er.Line, er.Span),
			Severity: protocol.DiagnosticSeverityError,
			Source:   "gnopls",
			Message:  er.Msg,
			Code:     er.Tool,
		})
	}

	return diagnostics, nil
}

func (s *server) publishDiagnostics(ctx context.Context, conn jsonrpc2.Conn, file *GnoFile, diagnostics []protocol.Diagnostic) error {
	return conn.Notify(
		ctx,
		protocol.MethodTextDocumentPublishDiagnostics,
		protocol.PublishDiagnosticsParams{
			URI:         file.URI,
			Diagnostics: diagnostics,
		},
	)
}
