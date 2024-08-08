package main

import (
	"bytes"
	"go/ast"
	"go/doc/comment"

	"go.lsp.dev/protocol"
)

func parseDocGroup(group *ast.CommentGroup) *protocol.MarkupContent {
	if group == nil || len(group.List) == 0 {
		return nil
	}

	var (
		parser  comment.Parser
		printer comment.Printer
	)

	str := group.Text()
	parsedDoc := parser.Parse(str)
	mdDoc := printer.Markdown(parsedDoc)
	mdDoc = bytes.TrimSuffix(mdDoc, []byte("\n"))
	return &protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: string(mdDoc),
	}
}
