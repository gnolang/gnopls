package main

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"go.lsp.dev/protocol"
)

const stringBuffSize = 128

var (
	astDocFields     = []string{"Doc", "Comment"}
	commentBlockType = reflect.TypeOf((*ast.CommentGroup)(nil))

	token2KindMapping = map[token.Token]protocol.CompletionItemKind{
		token.VAR:   protocol.CompletionItemKindVariable,
		token.CONST: protocol.CompletionItemKindConstant,
		token.TYPE:  protocol.CompletionItemKindClass,
	}
)

func declToCompletionItem(fset *token.FileSet, filter Filter, specGroup *ast.GenDecl) ([]protocol.CompletionItem, error) {
	if len(specGroup.Specs) == 0 {
		return nil, nil
	}

	blockKind, ok := token2KindMapping[specGroup.Tok]
	if !ok {
		return nil, fmt.Errorf("unsupported declaration token %q", specGroup.Tok)
	}

	// single-spec block declaration have documentation at the root of a block.
	// multi-spec blocks should use per-spec doc property.
	isDeclGroup := len(specGroup.Specs) > 1

	ctx := ParentCtx{
		decl:        specGroup,
		tokenKind:   blockKind,
		isDeclGroup: isDeclGroup,
		filter:      filter,
	}

	completions := make([]protocol.CompletionItem, 0, len(specGroup.Specs))
	for _, spec := range specGroup.Specs {
		switch t := spec.(type) {
		case *ast.TypeSpec:
			if !filter.allow(t.Name.String()) {
				continue
			}

			item, err := typeToCompletionItem(fset, ctx, t)
			if err != nil {
				return nil, err
			}

			completions = append(completions, item)
		case *ast.ValueSpec:
			items, err := valueToCompletionItem(fset, ctx, t)
			if err != nil {
				return nil, err
			}

			if len(items) == 0 {
				continue
			}

			completions = append(completions, items...)
		default:
			return nil, fmt.Errorf("unsupported declaration type %T", t)
		}
	}
	return completions, nil
}

type ParentCtx struct {
	decl        *ast.GenDecl
	tokenKind   protocol.CompletionItemKind
	isDeclGroup bool
	filter      Filter
}

func typeToCompletionItem(fset *token.FileSet, ctx ParentCtx, spec *ast.TypeSpec) (protocol.CompletionItem, error) {
	declCommentGroup := spec.Comment
	item := protocol.CompletionItem{
		Kind:             ctx.tokenKind,
		Label:            spec.Name.Name,
		InsertText:       spec.Name.Name,
		InsertTextFormat: protocol.InsertTextFormatPlainText,
	}

	isPrimitive := false
	switch spec.Type.(type) {
	case *ast.InterfaceType:
		item.Kind = protocol.CompletionItemKindInterface
	case *ast.StructType:
		item.InsertText = item.InsertText + "{}"
		item.Kind = protocol.CompletionItemKindStruct
	case *ast.Ident:
		isPrimitive = true
	}

	if !ctx.isDeclGroup {
		declCommentGroup = ctx.decl.Doc
	}

	if !isPrimitive {
		signature, err := typeToString(fset, ctx.decl)
		if err != nil {
			return item, fmt.Errorf("%w (type: %q)", err, item.Label)
		}

		item.Detail = signature
	}

	item.Documentation = parseDocGroup(declCommentGroup)
	return item, nil
}

func valueToCompletionItem(fset *token.FileSet, ctx ParentCtx, spec *ast.ValueSpec) ([]protocol.CompletionItem, error) {
	var blockDoc *protocol.MarkupContent
	if !ctx.isDeclGroup {
		blockDoc = parseDocGroup(spec.Doc)
	}

	items := make([]protocol.CompletionItem, 0, len(spec.Values))
	for _, val := range spec.Names {
		if !ctx.filter.allow(val.Name) {
			continue
		}

		item := protocol.CompletionItem{
			Kind:             ctx.tokenKind,
			Label:            val.Name,
			InsertText:       val.Name,
			InsertTextFormat: protocol.InsertTextFormatPlainText,
			Documentation:    blockDoc,
		}

		switch val.Name {
		case "true", "false":
		default:
			signature, err := typeToString(fset, val.Obj.Decl)
			if err != nil {
				return nil, fmt.Errorf("%w (value name: %s)", err, val.Name)
			}

			// declaration type is not present in value block.
			if signature != "" {
				signature = ctx.decl.Tok.String() + " " + signature
			}

			item.Detail = signature
		}

		items = append(items, item)
	}

	return items, nil
}

func funcToCompletionItem(fset *token.FileSet, format protocol.InsertTextFormat, fn *ast.FuncDecl) (item protocol.CompletionItem, err error) {
	isSnippet := format == protocol.InsertTextFormatSnippet
	item = protocol.CompletionItem{
		Label:            fn.Name.String(),
		Kind:             protocol.CompletionItemKindFunction,
		InsertTextFormat: format,
		InsertText:       buildFuncInsertStatement(fn, isSnippet),
		Documentation:    parseDocGroup(fn.Doc),
	}

	item.Detail, err = typeToString(fset, fn)
	if err != nil {
		return item, err
	}

	return item, nil
}

func buildFuncInsertStatement(decl *ast.FuncDecl, asSnippet bool) string {
	if !asSnippet {
		return decl.Name.String() + "()"
	}

	// snippet offsets start at 1
	offset := 1

	typ := decl.Type
	sb := new(strings.Builder)
	sb.Grow(stringBuffSize)
	sb.WriteString(decl.Name.String())
	offset = writeTypeParams(sb, offset, typ.TypeParams)
	sb.WriteString("(")
	writeParamsList(sb, offset, typ.Params)
	sb.WriteString(")")
	return sb.String()
}

func writeTypeParams(sb *strings.Builder, snippetOffset int, typeParams *ast.FieldList) int {
	if typeParams == nil || len(typeParams.List) == 0 {
		return snippetOffset
	}

	sb.WriteRune('[')
	offset := writeParamsList(sb, snippetOffset, typeParams)
	sb.WriteRune(']')
	return offset
}

func writeParamsList(sb *strings.Builder, snippetOffset int, params *ast.FieldList) int {
	if params == nil || len(params.List) == 0 {
		return snippetOffset
	}

	offset := snippetOffset
	for i, arg := range params.List {
		if i > 0 {
			sb.WriteString(", ")
		}

		for j, n := range arg.Names {
			if j > 0 {
				sb.WriteString(", ")
			}

			sb.WriteString("${")
			sb.WriteString(strconv.Itoa(offset))
			sb.WriteRune(':')
			sb.WriteString(n.String())
			sb.WriteRune('}')
			offset++
		}
	}

	return offset
}

func typeToString(fset *token.FileSet, decl any) (string, error) {
	// Remove comments block from AST node to keep only node body
	trimmedDecl := trimCommentBlock(decl)

	sb := new(strings.Builder)
	sb.Grow(stringBuffSize)
	err := printer.Fprint(sb, fset, trimmedDecl)
	if err != nil {
		return "", fmt.Errorf("can't generate type signature out of AST node %T: %w", trimmedDecl, err)
	}

	return sb.String(), nil
}

func trimCommentBlock(decl any) any {
	val := reflect.ValueOf(decl)
	isPtr := val.Kind() == reflect.Pointer
	if isPtr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return decl
	}

	dst := reflect.New(val.Type()).Elem()
	dst.Set(val)

	// *ast.FuncDecl, *ast.Object have Doc
	// *ast.Object and *ast.Indent might have Comment
	for _, fieldName := range astDocFields {
		field, ok := val.Type().FieldByName(fieldName)
		if ok && field.Type.AssignableTo(commentBlockType) {
			dst.FieldByIndex(field.Index).SetZero()
		}
	}

	if isPtr {
		dst = dst.Addr()
	}

	return dst.Interface()
}
