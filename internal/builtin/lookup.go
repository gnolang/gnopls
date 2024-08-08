package builtin

import (
	"strings"

	"go.lsp.dev/protocol"
)

//go:generate go run ../../tools/codegen-builtins -src ../../tools/gendata/builtin.go.txt -dest ./builtin_gen.go -omit Type,Type1,IntegerType,FloatType,ComplexType

// GetCompletions provides list of builtin symbols that has passed prefix in a name.
func GetCompletions(prefix string) []protocol.CompletionItem {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}

	key, remainder := getBucketKey(prefix)
	bucket, ok := buckets[key]
	if !ok {
		return nil
	}

	if remainder == 0 {
		return bucket
	}

	// most buckets contain only single value
	if len(bucket) == 1 && strings.HasPrefix(bucket[0].Label, prefix) {
		return bucket
	}

	var items []protocol.CompletionItem
	for _, item := range bucket {
		// TODO: use fuzzy find?
		if strings.HasPrefix(item.Label, prefix) {
			items = append(items, item)
		}
	}

	return items
}

func getBucketKey(str string) (rune, int) {
	runes := []rune(str)
	return runes[0], len(runes) - 1
}
