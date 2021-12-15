// Copyright 2021 D2iQ, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/utils/pointer"
)

func TestParseFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    SourceConfig
		wantErr bool
	}{{
		name: "empty",
		want: nil,
	}, {
		name: "single registry with no images",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{},
		},
	}, {
		name: "single registry with image with no tags",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image": nil,
				},
			},
		},
	}, {
		name: "single registry with image with single tag",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image": {"tag1"},
				},
			},
		},
	}, {
		name: "single registry with image with multiple tags",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image": {"tag1", "tag2", "tag3"},
				},
			},
		},
	}, {
		name: "single registry with multiple images with multiple tags",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image":  {"tag1", "tag2", "tag3"},
					"test-image2": {"tag3", "tag4", "tag5"},
					"test-image3": {"tag6", "tag7", "tag8"},
				},
			},
		},
	}, {
		name: "multiple registries with multiple images with multiple tags",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image":  {"tag1", "tag2", "tag3"},
					"test-image2": {"tag3", "tag4", "tag5"},
					"test-image3": {"tag6", "tag7", "tag8"},
				},
			}, "test.registry2.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image":  {"tag11", "tag21", "tag31"},
					"test-image5": {"tag32", "tag42", "tag52"},
					"test-image6": {"tag63", "tag73", "tag83"},
				},
			},
		},
	}, {
		name: "single registry with tls config",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				TLSVerify: pointer.Bool(false),
			},
		},
	}, {
		name: "multiple registries with tls config",
		want: SourceConfig{
			"test.registry.io": RegistrySyncConfig{
				TLSVerify: pointer.Bool(false),
			},
			"test.registry2.io": RegistrySyncConfig{
				TLSVerify: pointer.Bool(true),
			},
		},
	}}
	for ti := range tests {
		tt := tests[ti]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseFile(filepath.Join("testdata", strings.ReplaceAll(tt.name, " ", "_")+".yaml"))
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
