// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

type platform struct {
	os      string
	arch    string
	variant string
}

func (p platform) OS() string {
	return p.os
}

func (p platform) Arch() string {
	return p.arch
}

func (p platform) Variant() string {
	return p.variant
}

func (p platform) String() string {
	s := p.os + "/" + p.arch
	if p.variant != "" {
		s += "/" + p.variant
	}
	return s
}

// -- platformSlice Value.
type platformSliceValue struct {
	value   *[]platform
	changed bool
}

func newPlatformSlicesValue(val []platform, p *[]platform) *platformSliceValue {
	psv := new(platformSliceValue)
	psv.value = p
	*psv.value = val
	return psv
}

func readPlatformsAsCSV(val string) ([]platform, error) {
	if val == "" {
		return []platform{}, nil
	}
	stringReader := strings.NewReader(val)
	csvReader := csv.NewReader(stringReader)
	values, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	platforms := make([]platform, 0, len(values))
	for _, v := range values {
		p, err := parsePlatformString(v)
		if err != nil {
			return nil, err
		}
		platforms = append(platforms, p)
	}
	return platforms, nil
}

func writePlatformsAsCSV(vals []platform) (string, error) {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	strs := make([]string, 0, len(vals))
	for _, v := range vals {
		strs = append(strs, v.String())
	}
	err := w.Write(strs)
	if err != nil {
		return "", err
	}
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func parsePlatformString(s string) (platform, error) {
	splitVal := strings.Split(s, "/")
	if len(splitVal) < 2 || len(splitVal) > 3 {
		return platform{}, fmt.Errorf(
			"invalid platform specification: %s (required format: <os>/<arch>[/<variant>]",
			s,
		)
	}
	p := platform{os: splitVal[0], arch: splitVal[1]}
	if len(splitVal) == 3 {
		p.variant = splitVal[2]
	}
	return p, nil
}

var (
	_ pflag.Value      = &platformSliceValue{}
	_ pflag.SliceValue = &platformSliceValue{}
)

func (s *platformSliceValue) Set(val string) error {
	v, err := readPlatformsAsCSV(val)
	if err != nil {
		return err
	}
	if !s.changed {
		*s.value = v
	} else {
		*s.value = append(*s.value, v...)
	}
	s.changed = true
	return nil
}

func (s *platformSliceValue) Type() string {
	return "platformSlice"
}

func (s *platformSliceValue) String() string {
	str, _ := writePlatformsAsCSV(*s.value)
	return "[" + str + "]"
}

func (s *platformSliceValue) Append(val string) error {
	p, err := parsePlatformString(val)
	if err != nil {
		return err
	}
	*s.value = append(*s.value, p)
	return nil
}

func (s *platformSliceValue) Replace(val []string) error {
	ps := make([]platform, 0, len(val))
	for _, v := range val {
		p, err := parsePlatformString(v)
		if err != nil {
			return err
		}
		ps = append(ps, p)
	}
	*s.value = ps
	return nil
}

func (s *platformSliceValue) GetSlice() []string {
	strs := make([]string, 0, len(*s.value))
	for _, p := range *s.value {
		strs = append(strs, p.String())
	}
	return strs
}
