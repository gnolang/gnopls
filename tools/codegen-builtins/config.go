package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type StringSet map[string]struct{}

func (set StringSet) toList() []string {
	if len(set) == 0 {
		return nil
	}

	list := make([]string, 0, len(set))
	for val := range set {
		list = append(list, val)
	}

	return list
}

type Filter interface {
	allow(v string) bool
}

type NopFilter struct{}

func (_ NopFilter) allow(_ string) bool {
	return true
}

type DictFilter struct {
	isIgnoreList bool
	dict         StringSet
}

func (f DictFilter) allow(v string) bool {
	_, ok := f.dict[v]
	if f.isIgnoreList {
		ok = !ok
	}

	return ok
}

type Params struct {
	// OutFile is destination where generated Go file will be written.
	OutFile string

	// OutPackageName is package name specified in output Go file.
	OutPackageName string

	// SourceFile is source Go file which contains predefined symbols declarations.
	SourceFile string

	// Omit is list of symbols to skip. Opposite to Pick.
	Omit StringSet

	// Pick is a list of symbols to process. Opposite to Omit.
	Pick StringSet

	// EnableInsertSnippets enables snippet syntax for insertions in protocol.CompletionItem.InsertText.
	//
	// See: protocol.InsertTextFormat
	EnableInsertSnippets bool
}

func (p Params) GenContext() GenContext {
	return GenContext{
		PackageName: p.OutPackageName,
		SourcePath:  p.SourceFile,
		OmitSymbols: p.Omit.toList(),
		PickSymbols: p.Pick.toList(),
	}
}

func (p Params) withDefaults() (Params, error) {
	if p.OutFile == "" || p.OutPackageName != "" {
		return p, nil
	}

	absPath, err := filepath.Abs(p.OutFile)
	if err != nil {
		return p, fmt.Errorf("can't resolve absolute path: %w", err)
	}

	p.OutFile = absPath
	p.OutPackageName = filepath.Base(filepath.Dir(absPath))
	if p.OutPackageName == "" {
		p.OutPackageName = "builtin"
	}

	return p, nil
}

func (p Params) validate() error {
	if p.SourceFile == "" {
		return errors.New("missing source file")
	}

	if p.OutFile == "" {
		return errors.New("missing destination file name")
	}

	if _, err := os.Stat(p.SourceFile); err != nil {
		return fmt.Errorf("source file %q doesn't exist: %w", p.SourceFile, err)
	}

	if len(p.Omit) > 0 && len(p.Pick) > 0 {
		return errors.New("can't use both -pick and -omit flags together")
	}

	return nil
}

func (p Params) getFilter() Filter {
	if len(p.Omit) > 0 {
		return DictFilter{
			isIgnoreList: true,
			dict:         p.Omit,
		}
	}

	if len(p.Pick) > 0 {
		return DictFilter{
			dict: p.Pick,
		}
	}

	return NopFilter{}
}

func paramsFromFlags() (Params, error) {
	var params Params
	flag.StringVar(
		&params.OutFile, "dest", "",
		"Destination file name where generated Go file will be written.",
	)
	flag.StringVar(
		&params.OutPackageName, "pkg", "",
		"Package name specified in output Go file. By default is parent dir name.",
	)
	flag.StringVar(
		&params.SourceFile, "src", "",
		"Source Go file name with builtin definitions.",
	)
	flag.BoolVar(
		&params.EnableInsertSnippets, "enable-snippets", false,
		"Enables snippet syntax in InsertText. "+
			"Might not be supported by other LSP clients so use this with caution.",
	)

	var pickList, omitList string
	flag.StringVar(
		&pickList, "pick", "",
		"Comma-separated list of symbols to process. Any other symbols will be ignored.",
	)
	flag.StringVar(
		&omitList, "omit", "",
		"Comma-separated list of symbols to skip. Opposite to -pick flag.",
	)

	flag.Parse()

	params.Omit = setFromCSV(omitList)
	params.Pick = setFromCSV(pickList)
	return params.withDefaults()
}

func setFromCSV(str string) StringSet {
	if str == "" {
		return nil
	}

	dst := make(StringSet)
	for _, elem := range strings.Split(str, ",") {
		dst[strings.TrimSpace(elem)] = struct{}{}
	}

	return dst
}
