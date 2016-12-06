package main

// Source: https://github.com/google/go-github/blob/master/github/strings.go

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
)

// Stringify attempts to create a reasonable string representation of types.
// It does things like resolve pointers to their values and omits struct fields with nil values.
// This is taken from Stringify method of go-github.
// License can be found at https://github.com/google/go-github/blob/master/LICENSE
func Stringify(anyStruct interface{}) string {
	var buf bytes.Buffer
	v := reflect.ValueOf(anyStruct)
	stringifyValue(&buf, v)
	return buf.String()
}

// stringifyValue was heavily inspired by the goprotobuf library.
// see https://github.com/golang/protobuf
func stringifyValue(w io.Writer, val reflect.Value) {
	if val.Kind() == reflect.Ptr && val.IsNil() {
		w.Write([]byte("<nil>"))
		return
	}

	v := reflect.Indirect(val)

	switch v.Kind() {
	case reflect.String:
		fmt.Fprintf(w, `"%s"`, v)
	case reflect.Slice:
		w.Write([]byte{'['})
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				w.Write([]byte{' '})
			}

			stringifyValue(w, v.Index(i))
		}

		w.Write([]byte{']'})
		return
	case reflect.Struct:
		if v.Type().Name() != "" {
			w.Write([]byte(v.Type().String()))
		}

		w.Write([]byte{'{'})

		var sep bool
		for i := 0; i < v.NumField(); i++ {
			fv := v.Field(i)
			if fv.Kind() == reflect.Ptr && fv.IsNil() {
				continue
			}
			if fv.Kind() == reflect.Slice && fv.IsNil() {
				continue
			}

			if sep {
				w.Write([]byte(", "))
			} else {
				sep = true
			}

			w.Write([]byte(v.Type().Field(i).Name))
			w.Write([]byte{':'})
			stringifyValue(w, fv)
		}

		w.Write([]byte{'}'})
	default:
		if v.CanInterface() {
			fmt.Fprint(w, v.Interface())
		}
	}
}
