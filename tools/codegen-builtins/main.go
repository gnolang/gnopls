package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.lsp.dev/protocol"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal("ERROR: ", err)
	}
}

func mainErr() error {
	usageFunc := flag.Usage
	flag.Usage = func() {
		fmt.Print("codegen-builtin - Generates completion items for intrinsic Gno functions.\n\n")
		usageFunc()
	}

	params, err := paramsFromFlags()
	if err != nil {
		return err
	}

	if err := params.validate(); err != nil {
		return err
	}

	return run(params)
}

func run(p Params) (err error) {
	pkg, err := loadPackage(p)
	if err != nil {
		return err
	}

	items, err := collectCompletionItems(pkg)
	if err != nil {
		return err
	}

	return saveFile(p, items)
}

func collectCompletionItems(pkg PackageContext) ([]protocol.CompletionItem, error) {
	symbols := make([]protocol.CompletionItem, 0, len(pkg.funcs)+len(pkg.decls))

	for _, fn := range pkg.funcs {
		if !pkg.filter.allow(fn.Name.Name) {
			continue
		}

		item, err := funcToCompletionItem(pkg.fset, pkg.insertTextFormat, fn)
		if err != nil {
			return nil, fmt.Errorf("failed to process func %q: %w", fn.Name, err)
		}

		symbols = append(symbols, item)
	}

	for _, typeDecl := range pkg.decls {
		items, err := declToCompletionItem(pkg.fset, pkg.filter, typeDecl)
		if err != nil {
			return nil, err
		}

		symbols = append(symbols, items...)
	}

	return symbols, nil
}

func saveFile(p Params, items []protocol.CompletionItem) error {
	err := os.MkdirAll(filepath.Dir(p.OutFile), 0755)
	if err != nil {
		return fmt.Errorf("failed to create output parent directory: %w", err)
	}

	src, err := generateSourceFile(p.GenContext(), items)
	if err != nil {
		return fmt.Errorf("cannot generate Go file: %w", err)
	}

	f, err := os.OpenFile(p.OutFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	defer f.Close()
	if err := src.Render(f); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}
