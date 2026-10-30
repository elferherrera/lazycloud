package app

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const detailListMarker = "*"

type filterTerm struct {
	path  []string
	value string
}

func (t filterTerm) isDetail() bool {
	return t.path != nil
}

func hasDetailTerm(terms ...filterTerm) bool {
	for _, t := range terms {
		if t.isDetail() {
			return true
		}
	}
	return false
}

func parseFilterTerm(term string, samples []any) filterTerm {
	head, value, found := strings.Cut(term, " ")
	if !found {
		return filterTerm{value: term}
	}
	path, ok := splitDetailPath(head)
	if !ok || !detailPathValid(samples, path) {
		return filterTerm{value: term}
	}
	return filterTerm{path: path, value: value}
}

func splitDetailPath(head string) ([]string, bool) {
	parts := strings.Split(head, ".")
	path := make([]string, 0, len(parts))
	for i, part := range parts {
		if part == "" {
			return nil, false
		}
		if part == detailListMarker {
			if i == 0 || i == len(parts)-1 || path[len(path)-1] == detailListMarker {
				return nil, false
			}
		} else if strings.Contains(part, detailListMarker) {
			return nil, false
		}
		path = append(path, part)
	}
	return path, true
}

func detailPathValid(samples []any, path []string) bool {
	for _, sample := range samples {
		if detailPathResolves(reflect.ValueOf(sample), path) {
			return true
		}
	}
	return false
}

func detailPathResolves(v reflect.Value, path []string) bool {
	v = unwrapForValidate(v)
	if !v.IsValid() {
		return false
	}
	if len(path) == 0 {
		return true
	}
	if path[0] == detailListMarker {
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array && v.Kind() != reflect.Map {
			return false
		}
		return detailPathResolves(reflect.New(v.Type().Elem()).Elem(), path[1:])
	}
	switch v.Kind() {
	case reflect.Map:
		if len(path) == 1 {
			return true
		}
		return detailPathResolves(reflect.New(v.Type().Elem()).Elem(), path[1:])
	case reflect.Struct:
		for i := range v.Type().NumField() {
			if fieldNameMatches(v.Type().Field(i).Name, path[0]) {
				return detailPathResolves(v.Field(i), path[1:])
			}
		}
	}
	return false
}

func fieldNameMatches(field, name string) bool {
	if strings.EqualFold(field, name) {
		return true
	}
	if strings.HasSuffix(field, "s") && strings.EqualFold(field[:len(field)-1], name) {
		return true
	}
	return strings.EqualFold(field+"s", name)
}

func unwrapForValidate(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		v = reflect.New(v.Type().Elem()).Elem()
	}
	return v
}

func unwrap(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func rowMatches(row []string, res any, include filterTerm, excludes []filterTerm) bool {
	if !termMatches(row, res, include) {
		return false
	}
	for _, exclude := range excludes {
		if termMatches(row, res, exclude) {
			return false
		}
	}
	return true
}

func termMatches(row []string, res any, term filterTerm) bool {
	if term.isDetail() {
		return res != nil && detailTermMatches(res, term.path, term.value)
	}
	for _, val := range row {
		if strings.Contains(strings.ToLower(val), term.value) {
			return true
		}
	}
	return false
}

func detailTermMatches(res any, path []string, value string) bool {
	v := unwrap(reflect.ValueOf(res))
	if !v.IsValid() {
		return false
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			if detailMatch(v.Index(i), path, value) {
				return true
			}
		}
		return false
	}
	return detailMatch(v, path, value)
}

func detailMatch(v reflect.Value, path []string, value string) bool {
	if !v.IsValid() {
		return false
	}
	if len(path) == 0 {
		return strings.Contains(strings.ToLower(renderDetail(v)), value)
	}
	if path[0] == detailListMarker {
		v = unwrap(v)
		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				if detailMatch(v.Index(i), path[1:], value) {
					return true
				}
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				if detailMatch(v.MapIndex(k), path[1:], value) {
					return true
				}
			}
		}
		return false
	}
	field, ok := detailField(v, path[0])
	if !ok {
		return false
	}
	return detailMatch(field, path[1:], value)
}

func detailField(v reflect.Value, name string) (reflect.Value, bool) {
	v = unwrap(v)
	if !v.IsValid() {
		return reflect.Value{}, false
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.Type().NumField() {
			if fieldNameMatches(v.Type().Field(i).Name, name) {
				return v.Field(i), true
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			if strings.EqualFold(renderDetail(k), name) {
				return v.MapIndex(k), true
			}
		}
	}
	return reflect.Value{}, false
}

func renderDetail(v reflect.Value) string {
	v = unwrap(v)
	if !v.IsValid() {
		return ""
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Map:
		parts := make([]string, 0, v.Len())
		for _, k := range v.MapKeys() {
			parts = append(parts, renderDetail(k)+"="+renderDetail(v.MapIndex(k)))
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	case reflect.Slice, reflect.Array:
		parts := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			parts = append(parts, renderDetail(v.Index(i)))
		}
		return strings.Join(parts, ",")
	case reflect.Struct:
		parts := make([]string, 0, v.Type().NumField())
		for i := range v.Type().NumField() {
			parts = append(parts, v.Type().Field(i).Name+"="+renderDetail(v.Field(i)))
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	default:
		if v.CanInterface() {
			return fmt.Sprint(v.Interface())
		}
		return ""
	}
}
