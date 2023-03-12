// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package envconfig

import (
	"encoding/base64"
	"reflect"
	"strconv"
	"time"
)

// setBool parses the string as a boolean, and sets it to the value.
func setBool(v reflect.Value, s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	v.SetBool(b)
	return nil
}

// setDuration parses the string as a `time.Duration`, and sets it to the value.
func setDuration(v reflect.Value, s string) error {
	d, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	v.SetInt(int64(d))
	return nil
}

// setInt parses the string as an integer, and sets it to the value.
func setInt(v reflect.Value, s string) error {
	n, err := strconv.ParseInt(s, 0, v.Type().Bits())
	if err != nil {
		return err
	}
	v.SetInt(n)
	return nil
}

// setUint parses the string as an unsigned integer, and sets it to the value.
func setUint(v reflect.Value, s string) error {
	n, err := strconv.ParseUint(s, 0, v.Type().Bits())
	if err != nil {
		return err
	}
	v.SetUint(n)
	return nil
}

// setFloat parses the string as a floating point number, and sets it to the value.
func setFloat(v reflect.Value, s string) error {
	f, err := strconv.ParseFloat(s, v.Type().Bits())
	if err != nil {
		return err
	}
	v.SetFloat(f)
	return nil
}

// setBytes expects the string to be base64 encoded,
// which is decoded and set to the value.
func setBytes(v reflect.Value, s string) error {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	v.SetBytes(b)
	return nil
}
