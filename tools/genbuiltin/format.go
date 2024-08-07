package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/printer"
	"go/token"
	"strings"

	"go.lsp.dev/protocol"
)

const stringBuffSize = 128

func typeToCompletionItem(fset *token.FileSet, typ *doc.Type) protocol.CompletionItem {
	docStr := strings.TrimSpace(typ.Doc)
	return protocol.CompletionItem{
		Label:            typ.Name,
		Detail:           docStr,
		Kind:             protocol.CompletionItemKindClass,
		InsertTextFormat: protocol.InsertTextFormatPlainText,
		InsertText:       typ.Name,

		// TODO: parse Go-style code blocks into markdown.
		// See: https://github.com/x1unix/go-playground/blob/master/internal/analyzer/docfmt.go
		Documentation: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: docStr,
		},
	}
}

func funcToCompletionItem(fset *token.FileSet, fn *doc.Func) (item protocol.CompletionItem, err error) {
	item = protocol.CompletionItem{
		Label: fn.Name,
		Kind:  protocol.CompletionItemKindFunction,

		// Not all LSP clients support snippet mode.
		InsertTextFormat: protocol.InsertTextFormatPlainText,
		InsertText:       buildFuncInsertStatement(fn.Decl),

		// TODO: parse Go-style code blocks into markdown.
		// See: https://github.com/x1unix/go-playground/blob/master/internal/analyzer/docfmt.go
		Documentation: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: strings.TrimSpace(fn.Doc),
		},
	}

	item.Detail, err = typeToString(fset, fn.Decl)
	if err != nil {
		return item, err
	}

	return item, nil
}

func buildFuncInsertStatement(decl *ast.FuncDecl) string {
	typ := decl.Type
	sb := new(strings.Builder)
	sb.Grow(stringBuffSize)
	sb.WriteString(decl.Name.String())
	writeTypeParams(sb, typ.TypeParams)
	sb.WriteString("(")
	writeParamsList(sb, typ.Params)
	sb.WriteString(")")
	return sb.String()
}

func writeTypeParams(sb *strings.Builder, typeParams *ast.FieldList) {
	if typeParams == nil || len(typeParams.List) == 0 {
		return
	}

	sb.WriteRune('[')
	writeParamsList(sb, typeParams)
	sb.WriteRune(']')
}

func writeParamsList(sb *strings.Builder, params *ast.FieldList) {
	if params == nil || len(params.List) == 0 {
		return
	}

	for i, arg := range params.List {
		if i > 0 {
			sb.WriteString(", ")
		}
		for j, n := range arg.Names {
			if j > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString(n.String())
		}
	}
}

func typeToString(fset *token.FileSet, decl ast.Decl) (string, error) {
	sb := new(strings.Builder)
	sb.Grow(stringBuffSize)
	err := printer.Fprint(sb, fset, decl)
	if err != nil {
		return "", fmt.Errorf("can't generate type signature out of AST node %T: %w", decl, err)
	}

	return sb.String(), nil
}
