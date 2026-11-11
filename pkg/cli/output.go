package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Output formats. --json is the default; agents get stable snake_case
// keys on every format.
const (
	formatJSON   = "json"
	formatYAML   = "yaml"
	formatNDJSON = "ndjson"
	formatTable  = "table"
)

// toSnake converts an exported Go field name to snake_case: ProjectID
// -> project_id, MTU -> mtu, IPRange -> ip_range, SizeGB -> size_gb.
// The rule: underscore before an uppercase letter that follows a
// lowercase letter (word boundary), or that starts a word after an
// acronym (upper run whose next letter is lowercase).
func toSnake(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if !unicode.IsUpper(r) {
			b.WriteRune(r)
			continue
		}
		if i > 0 {
			prevLower := unicode.IsLower(runes[i-1])
			afterAcronym := i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1])
			if prevLower || afterAcronym {
				b.WriteRune('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// snakeValue rewrites struct field names to snake_case while leaving
// data untouched: map keys (VM labels, arbitrary user strings) and
// scalar values pass through unchanged, so agent-facing output gets a
// stable schema without corrupting payloads. The provider structs
// keep their tagless JSON encoding for the cache blobs; renaming
// happens only at this edge.
func snakeValue(v any) any {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339Nano)
	}
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Struct:
		rt := rv.Type()
		out := make(map[string]any, rv.NumField())
		for i := 0; i < rv.NumField(); i++ {
			if !rt.Field(i).IsExported() {
				continue
			}
			out[toSnake(rt.Field(i).Name)] = snakeValue(rv.Field(i).Interface())
		}
		return out
	case reflect.Ptr:
		if rv.IsNil() {
			return nil
		}
		return snakeValue(rv.Elem().Interface())
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		fallthrough
	case reflect.Array:
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = snakeValue(rv.Index(i).Interface())
		}
		return out
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = snakeValue(iter.Value().Interface())
		}
		return out
	default:
		return v
	}
}

// renderItems writes a list in the requested format. json prints one
// compact array; ndjson prints one object per line; yaml uses the same
// marshaller as the TUI :write command; table prints the TUI columns.
func renderItems(items []any, format string, def resourceDef, w io.Writer) error {
	switch format {
	case formatTable:
		rows := make([][]string, len(items))
		for i, item := range items {
			rows[i] = def.row(item)
		}
		return writeTable(def.headers, rows, w)
	case formatNDJSON:
		for _, item := range items {
			if err := writeOne(item, w); err != nil {
				return err
			}
		}
		return nil
	default:
		return writeStructured(snakeValue(items), format, w)
	}
}

// renderSingle writes one object; table prints its row under headers.
func renderSingle(item any, format string, def resourceDef, w io.Writer) error {
	if format == formatTable {
		return writeTable(def.headers, [][]string{def.row(item)}, w)
	}
	if format == formatNDJSON {
		return writeOne(item, w)
	}
	return writeStructured(snakeValue(item), format, w)
}

// writeOne prints a single object as one ndjson line.
func writeOne(item any, w io.Writer) error {
	encoded, err := json.Marshal(snakeValue(item))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

// writeStructured prints json (compact, one line) or yaml.
func writeStructured(v any, format string, w io.Writer) error {
	if format == formatYAML {
		encoded, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(encoded)
		return err
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}
