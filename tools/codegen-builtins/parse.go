package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

type PackageContext struct {
	fset   *token.FileSet
	decls  []*ast.GenDecl
	funcs  []*ast.FuncDecl
	filter Filter
}

func loadPackage(p Params) (PackageContext, error) {
	fset := token.NewFileSet()
	ctx := PackageContext{
		fset:   fset,
		filter: p.getFilter(),
	}

	pkgs, err := parser.ParseDir(fset, p.SourcePkgDir, nil, parser.ParseComments)
	if err != nil {
		return ctx, fmt.Errorf("can't parse source Go package: %w", err)
	}

	// Usually, "go/doc" can be used to collect funcs and type info but for some reason
	// it ignores most of declared functions in "builin.go"
	for name, pkg := range pkgs {
		if err := readPackage(&ctx, pkg); err != nil {
			return ctx, fmt.Errorf("can't parse package %q: %w", name, err)
		}
	}

	return ctx, nil
}

func readPackage(dst *PackageContext, pkg *ast.Package) error {
	for name, root := range pkg.Files {
		if err := readFile(dst, root); err != nil {
			return fmt.Errorf("error in file %q: %w", name, err)
		}
	}
	return nil
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
