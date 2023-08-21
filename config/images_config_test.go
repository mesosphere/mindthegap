// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestParseImagesFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    ImagesConfig
		wantErr bool
	}{{
		name: "empty",
		want: nil,
	}, {
		name: "single registry with no images",
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{},
		},
	}, {
		name: "single registry with image with no tags",
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image": nil,
				},
			},
		},
	}, {
		name: "single registry with image with single tag",
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image": {"tag1"},
				},
			},
		},
	}, {
		name: "single registry with image with multiple tags",
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image": {"tag1", "tag2", "tag3"},
				},
			},
		},
	}, {
		name: "single registry with multiple images with multiple tags",
		want: ImagesConfig{
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
		want: ImagesConfig{
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
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{
				TLSVerify: ptr.To(false),
			},
		},
	}, {
		name: "multiple registries with tls config",
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{
				TLSVerify: ptr.To(false),
			},
			"test.registry2.io": RegistrySyncConfig{
				TLSVerify: ptr.To(true),
			},
		},
	}, {
		name: "multiple registries with multiple images with multiple tags in plain text file",
		want: ImagesConfig{
			"test.registry.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image":  {"tag1", "tag3", "tag2"},
					"test-image2": {"tag3", "tag4", "tag5"},
					"test-image3": {"tag6", "tag7", "tag8"},
				},
			}, "test.registry2.io": RegistrySyncConfig{
				Images: map[string][]string{
					"test-image":  {"tag21", "tag11", "tag31"},
					"test-image5": {"tag32", "tag42", "tag52"},
					"test-image6": {"tag63", "tag73", "tag83"},
				},
			}, "docker.io": RegistrySyncConfig{
				Images: map[string][]string{
					"plain/image":    {"tag"},
					"library/image2": {"tag2"},
				},
			},
		},
	}}
	for ti := range tests {
		tt := tests[ti]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ext := "yaml"
			if strings.HasSuffix(tt.name, "in plain text file") {
				ext = "txt"
			}
			got, err := ParseImagesConfigFile(
				filepath.Join("testdata", "images", strings.ReplaceAll(tt.name, " ", "_")+"."+ext),
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

func TestMergeConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  *ImagesConfig
		with ImagesConfig
		want *ImagesConfig
	}{{
		name: "empty",
		want: nil,
	}, {
		name: "empty to merge",
		src: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
	}, {
		name: "empty from merge",
		with: ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
	}, {
		name: "distinct registries",
		src: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
		with: ImagesConfig{
			"b": RegistrySyncConfig{
				Images: map[string][]string{"2": {"v2"}},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
			"b": RegistrySyncConfig{
				Images: map[string][]string{"2": {"v2"}},
			},
		},
	}, {
		name: "duplicate registries with same configuration",
		src: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
		with: ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
	}, {
		name: "duplicate registries with extra tags",
		src: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1"}},
			},
		},
		with: ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1", "v2"}},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1", "v2"}},
			},
		},
	}, {
		name: "duplicate registries with extra image",
		src: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{"1": {"v1", "v3"}},
			},
		},
		with: ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{
					"1": {"v1", "v2"},
					"2": {"v3"},
				},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{
					"1": {"v1", "v2", "v3"},
					"2": {"v3"},
				},
			},
		},
	}, {
		name: "duplicate registries with extra image",
		src: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{
					"1": {"v1", "v3"},
					"2": {"v3"},
				},
			},
		},
		with: ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{
					"1": {"v1", "v2", "v4"},
				},
			},
		},
		want: &ImagesConfig{
			"a": RegistrySyncConfig{
				Images: map[string][]string{
					"1": {"v1", "v2", "v3", "v4"},
					"2": {"v3"},
				},
			},
		},
	}}

	for ti := range tests {
		tt := tests[ti]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.src.Merge(tt.with)
			assert.Equal(t, tt.want, got)
		})
	}
}
