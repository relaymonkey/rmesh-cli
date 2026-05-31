package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// ApplySetPaths mutates cfg by applying dotted-path overrides in the form
// "path=value". Paths follow the yaml tags on Config (snake_case), and map
// fields (currently just labels) accept "labels.<key>=<value>".
//
// Unknown paths are hard errors so typos don't silently no-op. See `D-220`.
func ApplySetPaths(cfg *Config, pairs []string) error {
	for _, p := range pairs {
		path, value, ok := strings.Cut(p, "=")
		if !ok {
			return fmt.Errorf("--set %q: expected path=value", p)
		}
		path = strings.TrimSpace(path)
		if path == "" {
			return fmt.Errorf("--set %q: empty path", p)
		}
		if err := setOnePath(cfg, strings.Split(path, "."), value); err != nil {
			return fmt.Errorf("--set %s: %w", path, err)
		}
	}
	return nil
}

func setOnePath(cfg *Config, segments []string, value string) error {
	v := reflect.ValueOf(cfg).Elem()
	for i, seg := range segments {
		switch v.Kind() {
		case reflect.Struct:
			field, ok := fieldByYAMLTag(v, seg)
			if !ok {
				return fmt.Errorf("unknown field %q", seg)
			}
			if i == len(segments)-1 {
				return assignScalar(field, value)
			}
			v = field
		case reflect.Map:
			if i != len(segments)-1 {
				return fmt.Errorf("path descends into map at %q", seg)
			}
			if v.Type().Key().Kind() != reflect.String || v.Type().Elem().Kind() != reflect.String {
				return fmt.Errorf("only map[string]string is supported")
			}
			if v.IsNil() {
				v.Set(reflect.MakeMap(v.Type()))
			}
			v.SetMapIndex(reflect.ValueOf(seg), reflect.ValueOf(value))
			return nil
		default:
			return fmt.Errorf("cannot descend into %s at %q", v.Kind(), seg)
		}
	}
	return nil
}

// fieldByYAMLTag returns the struct field whose yaml tag (first comma-separated
// token) equals name. Falls back to lowercase field name.
func fieldByYAMLTag(v reflect.Value, name string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("yaml")
		if tag != "" {
			if comma := strings.Index(tag, ","); comma >= 0 {
				tag = tag[:comma]
			}
		}
		if tag == "" {
			tag = strings.ToLower(f.Name)
		}
		if tag == name {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func assignScalar(field reflect.Value, raw string) error {
	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return assignScalar(field.Elem(), raw)
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("expected bool, got %q", raw)
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("expected duration, got %q", raw)
			}
			field.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("expected int, got %q", raw)
		}
		field.SetInt(n)
	default:
		return fmt.Errorf("unsupported field kind %s", field.Kind())
	}
	return nil
}
