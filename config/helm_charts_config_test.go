// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/utils/pointer"
)

func TestParseHelmChartsFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    HelmChartsConfig
		wantErr bool
	}{{
		name: "empty",
		want: HelmChartsConfig{},
	}, {
		name: "single repository with no charts",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {},
			},
		},
	}, {
		name: "single repository with chart with no requested versions",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					Charts: map[string][]string{
						"test-chart": nil,
					},
				},
			},
		},
	}, {
		name: "single repository with chart with single version",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					Charts: map[string][]string{
						"test-chart": {"v1.2.3"},
					},
				},
			},
		},
	}, {
		name: "single repository with chart with multiple versions",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					Charts: map[string][]string{
						"test-chart": {"v1.2.3", "v2.4.6", "v3.6.9"},
					},
				},
			},
		},
	}, {
		name: "single repository with multiple charts with multiple versions",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					Charts: map[string][]string{
						"test-chart":  {"v1.2.3", "v2.4.6", "v3.6.9"},
						"test-chart2": {"v3.6.9", "v4.8.12", "v5.10.15"},
						"test-chart3": {"v6.12.18", "v7.14.21", "v8.16.24"},
					},
				},
			},
		},
	}, {
		name: "multiple repositories with multiple charts with multiple versions",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					Charts: map[string][]string{
						"test-chart":  {"v1.2.3", "v2.4.6", "v3.6.9"},
						"test-chart2": {"v3.6.9", "v4.8.12", "v5.10.15"},
						"test-chart3": {"v6.12.18", "v7.14.21", "v8.16.24"},
					},
				}, "test.repository2.io": {
					Charts: map[string][]string{
						"test-chart":  {"v1.2.31", "v2.4.61", "v3.6.91"},
						"test-chart5": {"v3.6.92", "v4.8.122", "v5.10.152"},
						"test-chart6": {"v63.126.189", "v73.146.219", "v8.16.243"},
					},
				},
			},
		},
	}, {
		name: "single repository with tls config",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					TLSVerify: pointer.Bool(false),
				},
			},
		},
	}, {
		name: "multiple repositories with tls config",
		want: HelmChartsConfig{
			Repositories: map[string]HelmRepositorySyncConfig{
				"test.repository.io": {
					TLSVerify: pointer.Bool(false),
				},
				"test.repository2.io": {
					TLSVerify: pointer.Bool(true),
				},
			},
		},
	}}
	for ti := range tests {
		tt := tests[ti]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ext := "yaml"
			got, err := ParseHelmChartsConfigFile(
				filepath.Join(
					"testdata",
					"helmcharts",
					strings.ReplaceAll(tt.name, " ", "_")+"."+ext,
				),
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
