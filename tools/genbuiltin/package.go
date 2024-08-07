package main

import (
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"path/filepath"
)

type PackageContext struct {
	fset   *token.FileSet
	doc    *doc.Package
	filter Filter
}

func loadPackage(p Params) (ctx PackageContext, err error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, p.SourcePkgDir, nil, parser.ParseComments)
	if err != nil {
		return ctx, fmt.Errorf("can't parse source Go package: %w", err)
	}

	pkgName := filepath.Base(p.SourcePkgDir)
	pkg, ok := pkgs[pkgName]
	if !ok {
		return ctx, fmt.Errorf("dir name (%s) differs from Go package name", pkgName)
	}

	pkgDoc := doc.New(pkg, pkgName, doc.AllDecls)
	return PackageContext{
		doc:    pkgDoc,
		fset:   fset,
		filter: p.getFilter(),
	}, nil
}
