package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"go.lsp.dev/protocol"
)

type PackageContext struct {
	fset             *token.FileSet
	decls            []*ast.GenDecl
	funcs            []*ast.FuncDecl
	insertTextFormat protocol.InsertTextFormat
	filter           Filter
}

func loadPackage(p Params) (PackageContext, error) {
	fset := token.NewFileSet()
	ctx := PackageContext{
		fset:             fset,
		filter:           p.getFilter(),
		insertTextFormat: protocol.InsertTextFormatPlainText,
	}

	if p.EnableInsertSnippets {
		ctx.insertTextFormat = protocol.InsertTextFormatSnippet
	}

	root, err := parser.ParseFile(fset, p.SourceFile, nil, parser.ParseComments)
	if err != nil {
		return ctx, fmt.Errorf("can't parse source Go file: %w", err)
	}

	// Usually, "go/doc" can be used to collect funcs and type info but for some reason
	// it ignores most of declared functions in "builin.go"
	//
	// Also "go/doc" requires to parse a whole directory, but we should be able to deal
	// with non *.go files to keep them away from "go build".
	if err := readFile(&ctx, root); err != nil {
		return ctx, err
	}

	return ctx, nil
}

func readFile(dst *PackageContext, root *ast.File) error {
	for _, decl := range root.Decls {
		switch t := decl.(type) {
		case *ast.FuncDecl:
			dst.funcs = append(dst.funcs, t)
		case *ast.GenDecl:
			if t.Tok != token.IMPORT {
				dst.decls = append(dst.decls, t)
			}
		default:
			return fmt.Errorf("unsupported block type %T at %d", t, t.Pos())
		}
	}

	return nil
}
