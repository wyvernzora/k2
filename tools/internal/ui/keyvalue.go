package ui

import (
	"fmt"
	"strings"
)

// KV is one key/value pair for KeyValues blocks.
type KV struct {
	Key, Value string
}

// KeyValues prints aligned key/value pairs.
func (r *Reporter) KeyValues(pairs ...KV) {
	if r == nil || r.out == nil || len(pairs) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.plain {
		for _, p := range pairs {
			fmt.Fprintf(r.out, "k2-tools: %s: %s\n", p.Key, p.Value)
		}
		return
	}
	maxKey := 0
	for _, p := range pairs {
		if len(p.Key) > maxKey {
			maxKey = len(p.Key)
		}
	}
	for _, p := range pairs {
		key := KeyStyle.Render(fmt.Sprintf("%-*s", maxKey, p.Key))
		fmt.Fprintf(r.out, "  %s   %s\n", key, ValueStyle.Render(p.Value))
	}
}

// KeyValuef is a single-pair convenience wrapper around KeyValues.
func (r *Reporter) KeyValuef(key string, format string, args ...any) {
	value := fmt.Sprintf(format, args...)
	r.KeyValues(KV{Key: key, Value: value})
}

// RenderKeyValue returns the styled `<key>: <value>` string for one row.
func RenderKeyValue(key, value string) string {
	if !strings.HasSuffix(key, ":") {
		key += ":"
	}
	return KeyStyle.Render(key) + " " + ValueStyle.Render(value)
}
