package main

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"reflect"
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

	switch spec.Type.(type) {
	case *ast.InterfaceType:
		item.Kind = protocol.CompletionItemKindInterface
	case *ast.StructType:
		item.InsertText = item.InsertText + "{}"
		item.Kind = protocol.CompletionItemKindStruct
	}

	if !ctx.isDeclGroup {
		declCommentGroup = ctx.decl.Doc
	}

	signature, err := typeToString(fset, ctx.decl)
	if err != nil {
		return item, fmt.Errorf("%w (type: %q)", err, item.Label)
	}

	item.Detail = signature
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

		signature, err := typeToString(fset, val.Obj.Decl)
		if err != nil {
			return nil, fmt.Errorf("%w (value name: %s)", err, val.Name)
		}

		// declaration type is not present in value block.
		if signature != "" {
			signature = ctx.decl.Tok.String() + " " + signature
		}
		item := protocol.CompletionItem{
			Kind:             ctx.tokenKind,
			Label:            val.Name,
			InsertText:       val.Name,
			Detail:           signature,
			InsertTextFormat: protocol.InsertTextFormatPlainText,
			Documentation:    blockDoc,
		}

		items = append(items, item)
	}

	return items, nil
}

func funcToCompletionItem(fset *token.FileSet, fn *ast.FuncDecl) (item protocol.CompletionItem, err error) {
	item = protocol.CompletionItem{
		Label: fn.Name.String(),
		Kind:  protocol.CompletionItemKindFunction,

		// Not all LSP clients support snippet mode.
		InsertTextFormat: protocol.InsertTextFormatPlainText,
		InsertText:       buildFuncInsertStatement(fn),
		Documentation:    parseDocGroup(fn.Doc),
	}

	item.Detail, err = typeToString(fset, fn)
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
