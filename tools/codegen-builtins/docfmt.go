package main

import (
	"go/ast"
	"strings"

	"go.lsp.dev/protocol"
)

const (
	newLineChar = '\n'
	tabChar     = '\t'
)

var (
	mdCodeTag     = []byte("```\n")
	spaceIdent    = "  "
	spaceIdentLen = len(spaceIdent)
)

func isDocLine(line string) bool {
	// Source code in Go usually doc starts with tab char
	if line[0] == tabChar {
		return true
	}

	// Workaround for some packages with double space as doc indent (like "net/http")
	if (len(line) > spaceIdentLen) && (line[:spaceIdentLen] == spaceIdent) {
		return true
	}

	return false
}

func trimCommentSlash(str string) string {
	text, ok := strings.CutPrefix(str, "// ")
	if !ok {
		text, ok = strings.CutPrefix(str, "//")
	}

	if !ok {
		text = str
	}

	return text
}

// parseDocGroup parses declaration doc comment group and returns markdown-formatted documentation.
//
// Function respects Go-style code blocks, see: https://tip.golang.org/doc/comment.
func parseDocGroup(group *ast.CommentGroup) *protocol.MarkupContent {
	if group == nil || len(group.List) == 0 {
		return nil
	}

	w := strings.Builder{}
	docStart := false

	for _, block := range group.List {
		line := trimCommentSlash(block.Text)
		if len(line) == 0 {
			w.WriteRune(newLineChar)
			continue
		}

		// Source code in Go doc starts with tab char
		if isDocLine(line) {
			if !docStart {
				// Put markdown code section
				// if we met first source code line
				docStart = true
				w.Write(mdCodeTag)
			}

			w.WriteString(line)
			w.WriteRune(newLineChar)
			continue
		}

		// Else - regular text
		if docStart {
			// Terminate code block if previous
			// was open and not terminated
			docStart = false
			w.Write(mdCodeTag)
		}

		w.WriteString(line)
		w.WriteRune(newLineChar)
	}

	if docStart {
		// close markdown code block if wasn't closed
		w.Write(mdCodeTag)
	}

	str := w.String()
	str, _ = strings.CutSuffix(str, "\n")

	return &protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: str,
	}
}
