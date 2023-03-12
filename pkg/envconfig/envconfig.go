// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package envconfig

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrInvalidSpec      = errors.New("invalid spec")
	ErrMissingRequired  = errors.New("missing required value")
	ErrUnknownFieldType = errors.New("unknown field type")
	ErrInvalidMapItem   = errors.New("invalid map item")
)

type Setter interface {
	Set(s string) error
}

func MustProcess(spec any, prefix ...string) {
	if err := Process(spec, prefix...); err != nil {
		panic(err)
	}
}

func Process(spec any, prefix ...string) error {
	v := reflect.ValueOf(spec)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("not a pointer: %w", ErrInvalidSpec)
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("not a struct: %w", ErrInvalidSpec)
	}
	var p string
	if len(prefix) > 0 {
		p = prefix[0]
	}
	return processStruct(p, v)
}

func processStruct(prefix string, spec reflect.Value) error {
	types := spec.Type()
	for n := 0; n < spec.NumField(); n++ {
		field := spec.Field(n)
		if !field.CanSet() {
			continue
		}

		ftype := types.Field(n)
		name := ftype.Tag.Get("envconfig")

		if isRecursable(field) {
			if err := processStruct(prefix+name, field); err != nil {
				return err
			}
			continue
		}

		// If the `envconfig` tag isn't specified on the field,
		// we do not care about it.
		if name == "" {
			continue
		}

		value, err := getValueFor(prefix+name, ftype)
		if err != nil {
			return err
		}

		// No value to set, and the field is not required, so jump to the next one.
		if value == "" {
			continue
		}

		if err := processField(field, value); err != nil {
			return fmt.Errorf("%s: %w", prefix+name, err)
		}
	}
	return nil
}

// isRecursable checks if the value has a type we should recurse into, i.e. it
// is a struct, but it does not fulfil any of the unmarshable interfaces.
func isRecursable(v reflect.Value) bool {
	// If the value isn't a struct, we won't attempt to recurse into it.
	if v.Kind() != reflect.Struct {
		return false
	}

	// At this point we know we have a struct, but can we access it as an
	// interface? If not, we know we should recurse into it.
	if !v.CanInterface() {
		return true
	}

	// Does the interface match any of the unmarshable interfaces? If so, we
	// should *not* recurse into the struct.
	if isUnmarshable(v.Interface()) {
		return false
	}

	// The interface did not match the ones we care about, but if the value is
	// a pointer type, we also need to check if that implements the interfaces.
	return !v.CanAddr() || !isUnmarshable(v.Addr().Interface())
}

// isUnmarshable checks if the value implements any of the unmarshable
// interfaces we care about.
func isUnmarshable(v any) bool {
	switch v.(type) {
	case Setter, encoding.TextUnmarshaler, encoding.BinaryUnmarshaler:
		return true
	default:
		return false
	}
}

func getValueFor(name string, field reflect.StructField) (string, error) {
	if value := os.Getenv(name); value != "" {
		return value, nil
	}
	if def := field.Tag.Get("default"); def != "" {
		return def, nil
	}
	if isTrue(field.Tag.Get("required")) {
		return "", fmt.Errorf("%s: %w", name, ErrMissingRequired)
	}
	return "", nil
}

func processField(field reflect.Value, value string) error {
	if field.CanInterface() {
		if done, err := processUnmarshable(field, value); done {
			return err
		}
		if field.CanAddr() {
			if done, err := processUnmarshable(field.Addr(), value); done {
				return err
			}
		}
	}

	ftype := field.Type()
	if ftype.Kind() == reflect.Pointer {
		ftype = ftype.Elem()
		if field.IsNil() {
			field.Set(reflect.New(ftype))
		}
		field = field.Elem()
	}

	switch ftype.Kind() {
	case reflect.String:
		field.SetString(value)
		return nil
	case reflect.Bool:
		return setBool(field, value)
	case reflect.Int64:
		if isDuration(ftype) {
			return setDuration(field, value)
		}
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return setInt(field, value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUint(field, value)
	case reflect.Float32, reflect.Float64:
		return setFloat(field, value)
	case reflect.Slice:
		if isBytes(ftype) {
			return setBytes(field, value)
		}
		return processSlice(field, value)
	case reflect.Map:
		return processMap(field, value)
	default:
		return ErrUnknownFieldType
	}
}

func processUnmarshable(field reflect.Value, value string) (bool, error) {
	switch x := field.Interface().(type) {
	case Setter:
		return true, x.Set(value)
	case encoding.TextUnmarshaler:
		return true, x.UnmarshalText([]byte(value))
	case encoding.BinaryUnmarshaler:
		return true, x.UnmarshalBinary([]byte(value))
	default:
		return false, nil
	}
}

func processSlice(field reflect.Value, value string) error {
	var values []string
	if strings.TrimSpace(value) != "" {
		values = strings.Split(value, ",")
	}
	s := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, v := range values {
		if err := processField(s.Index(i), v); err != nil {
			return err
		}
	}
	field.Set(s)
	return nil
}

func processMap(field reflect.Value, value string) error {
	t := field.Type()
	m := reflect.MakeMap(t)
	if strings.TrimSpace(value) != "" {
		pairs := strings.Split(value, ";")
		for _, pair := range pairs {
			kv := strings.Split(pair, ":")
			if len(kv) != 2 {
				return fmt.Errorf("%w: %q", ErrInvalidMapItem, pair)
			}

			k := reflect.New(t.Key()).Elem()
			if err := processField(k, kv[0]); err != nil {
				return err
			}

			v := reflect.New(t.Elem()).Elem()
			if err := processField(v, kv[1]); err != nil {
				return err
			}

			m.SetMapIndex(k, v)
		}
	}
	field.Set(m)
	return nil
}

func isDuration(t reflect.Type) bool {
	return t.PkgPath() == "time" && t.Name() == "Duration"
}

func isBytes(t reflect.Type) bool {
	return t.Elem().Kind() == reflect.Uint8
}

func isTrue(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
