package tools

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type Token struct {
	Offset uint32 `json:"offset"`
	Line   uint32 `json:"line"`
	Column uint32 `json:"column"`
}

type Issue struct {
	Rule       string `json:"rule"`
	Category   string `json:"category"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
	Note       string `json:"note"`
	Start      Token  `json:"start"`
	End        Token  `json:"end"`
	Confidence int    `json:"confidence"`
}

type FileIssues map[string][]Issue

func Lint(ctx context.Context, conn jsonrpc2.Conn, uri protocol.DocumentURI) error {
	tempFile, err := os.CreateTemp("", "temp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	path := uri.Filename()
	cmd := exec.Command("tlin", "-json-output", tempFile.Name(), path)

	if err := cmd.Run(); err != nil {
		return err
	}

	// read the temp file
	content, err := io.ReadAll(tempFile)
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return nil
	}

	var data FileIssues
	err = json.Unmarshal(content, &data)
	if err != nil {
		return err
	}

	// send the diagnostics
	for _, issues := range data {
		diagnostics := make([]protocol.Diagnostic, len(issues))
		for i, issue := range issues {
			diagnostics[i] = protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      issue.Start.Line,
						Character: issue.Start.Column,
					},
					End: protocol.Position{
						Line:      issue.End.Line,
						Character: issue.End.Column,
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
	}
}
