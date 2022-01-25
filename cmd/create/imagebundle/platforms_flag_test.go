// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"fmt"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

const argfmt = "--ps=%s"

func setUpPSFlagSet(psp *[]platform) *pflag.FlagSet {
	f := pflag.NewFlagSet("test", pflag.ContinueOnError)
	f.Var(newPlatformSlicesValue(
		[]platform{}, psp),
		"ps", "Command separated list!")
	return f
}

func setUpPSFlagSetWithDefault(psp *[]platform) *pflag.FlagSet {
	f := pflag.NewFlagSet("test", pflag.ContinueOnError)
	f.Var(newPlatformSlicesValue(
		[]platform{
			{os: "defaultos1", arch: "defaultarch1"},
			{os: "defaultos2", arch: "defaultarch2", variant: "defaultvariant2"},
		}, psp),
		"ps", "Command separated list!")
	return f
}

func TestEmptyPS(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	getPS := f.Lookup("ps").Value
	// Empty value expected to be `[]``.
	if len(getPS.String()) != 2 {
		t.Fatalf("got ps %v with len=%d but expected length=2", getPS, len(getPS.String()))
	}
}

func TestEmptyPSValue(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)
	err := f.Parse([]string{"--ps="})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	getPS := f.Lookup("ps").Value
	if len(getPS.String()) != 2 {
		t.Fatalf("got ps %v with len=%d but expected length=2", getPS, len(getPS.String()))
	}
}

func TestPS(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	vals := []platform{
		{os: "linux", arch: "amd64"},
		{os: "linux", arch: "arm64"},
		{os: "windows", arch: "amd64"},
		{os: "darwin", arch: "arm64", variant: "v8"},
	}
	s, err := writePlatformsAsCSV(vals)
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	arg := fmt.Sprintf(argfmt, s)
	err = f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ps {
		if vals[i] != v {
			t.Fatalf("expected ps[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	getPS := f.Lookup("ps").Value.(*platformSliceValue)
	for i, v := range *getPS.value {
		if vals[i] != v {
			t.Fatalf("expected ps[%d] to be %s from Lookup but got: %s", i, vals[i], v)
		}
	}
}

func TestPSDefault(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSetWithDefault(&ps)

	vals := []platform{
		{os: "defaultos1", arch: "defaultarch1"},
		{os: "defaultos2", arch: "defaultarch2", variant: "defaultvariant2"},
	}

	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ps {
		if vals[i] != v {
			t.Fatalf("expected ps[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	getPS := f.Lookup("ps").Value.(*platformSliceValue)
	for i, v := range *getPS.value {
		if vals[i] != v {
			t.Fatalf("expected ps[%d] to be %s from Lookup but got: %s", i, vals[i], v)
		}
	}
}

func TestSSWithDefault(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSetWithDefault(&ps)

	vals := []platform{
		{os: "linux", arch: "amd64"},
		{os: "linux", arch: "arm64"},
		{os: "windows", arch: "amd64"},
		{os: "darwin", arch: "arm64", variant: "v8"},
	}
	s, err := writePlatformsAsCSV(vals)
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	arg := fmt.Sprintf(argfmt, s)
	err = f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ps {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	getPS := f.Lookup("ps").Value.(*platformSliceValue)
	for i, v := range *getPS.value {
		if vals[i] != v {
			t.Fatalf("expected ps[%d] to be %s from Lookup but got: %s", i, vals[i], v)
		}
	}
}

func TestSSCalledTwice(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	in := []string{"linux/amd64", "darwin/arm64/v8,linux/arm/v7"}
	expected := []platform{
		{os: "linux", arch: "amd64"},
		{os: "darwin", arch: "arm64", variant: "v8"},
		{os: "linux", arch: "arm", variant: "v7"},
	}

	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(ps) {
		t.Fatalf("expected number of ss to be %d but got: %d", len(expected), len(ps))
	}
	for i, v := range ps {
		if expected[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	values := f.Lookup("ps").Value.(*platformSliceValue)

	if len(expected) != len(*values.value) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(ps))
	}
	for i, v := range *values.value {
		if expected[i] != v {
			t.Fatalf("expected got ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestSSWithComma(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	in := []string{`"linux/amd64"`, `"windows/amd64"`, `"darwin/arm64/v8",linux/arm/v7`}
	expected := []platform{
		{os: "linux", arch: "amd64"},
		{os: "windows", arch: "amd64"},
		{os: "darwin", arch: "arm64", variant: "v8"},
		{os: "linux", arch: "arm", variant: "v7"},
	}
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	arg3 := fmt.Sprintf(argfmt, in[2])
	err := f.Parse([]string{arg1, arg2, arg3})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(ps) {
		t.Fatalf("expected number of ps to be %d but got: %d", len(expected), len(ps))
	}
	for i, v := range ps {
		if expected[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	values := f.Lookup("ps").Value.(*platformSliceValue)

	if len(expected) != len(*values.value) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(*values.value))
	}
	for i, v := range *values.value {
		if expected[i] != v {
			t.Fatalf("expected got ps[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestPSAsSliceValue(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	in := []string{"linux/amd64", "darwin/arm64/v8"}
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	require.NoError(t, f.Parse([]string{arg1, arg2}), "error parsing flags")

	f.VisitAll(func(f *pflag.Flag) {
		if val, ok := f.Value.(pflag.SliceValue); ok {
			_ = val.Replace([]string{"windows/arm/v7"})
		}
	})
	expectedPlatform := platform{os: "windows", arch: "arm", variant: "v7"}
	require.ElementsMatch(t, []platform{expectedPlatform}, ps, "Expected ps to be overwritten with 'windows/arm/v7'")
}

func TestPSGetSlice(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	in := []string{"linux/amd64", "darwin/arm64/v8"}
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	require.NoError(t, f.Parse([]string{arg1, arg2}), "error parsing flags")

	require.ElementsMatch(t,
		in,
		f.Lookup("ps").Value.(pflag.SliceValue).GetSlice(),
		"incorrect platforms",
	)
}

func TestPSAppend(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	in := []string{"linux/amd64", "darwin/arm64/v8"}
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	require.NoError(t, f.Parse([]string{arg1, arg2}), "error parsing flags")

	require.NoError(t, f.Lookup("ps").Value.(pflag.SliceValue).Append("windows/i386"), "error appending to platforms")
	require.ElementsMatch(t,
		append(in, "windows/i386"),
		f.Lookup("ps").Value.(pflag.SliceValue).GetSlice(),
		"incorrect platforms",
	)
}

func TestPSInvalidPlatform(t *testing.T) {
	t.Parallel()
	var ps []platform
	f := setUpPSFlagSet(&ps)

	in := []string{"wibble"}
	arg1 := fmt.Sprintf(argfmt, in[0])
	require.EqualError(t, f.Parse([]string{arg1}),
		`invalid argument "wibble" for "--ps" flag: invalid platform specification: `+
			`wibble (required format: <os>/<arch>[/<variant>]`,
		"expected error parsing flags",
	)
}
