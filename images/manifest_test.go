// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package images

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var busyboxIndexManifest = v1.IndexManifest{
	Manifests: []v1.Descriptor{{
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "907ca53d7e2947e849b839b1cd258c98fd3916c60f2e6e70c30edbf741ab6754",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "amd64"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "dde8e930c7b6a490f728e66292bc9bce42efc9bbb5278bae40e4f30f6e00fe8c",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v5"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "4ff685e2bcafdab0d2a9b15cbfd9d28f5dfe69af97e3bb1987ed483b0abf5a99",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v6"},
		Size:      527,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "77ed5ebc3d9d48581e8afcb75b4974978321bd74f018613483570fcd61a15de8",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "5e42fbc46b177f10319e8937dd39702e7891ce6d8a42d60c1b4f433f94200bd2",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "1c8bbeaff20b74c3918ae3da99db0f0d8563adb33fcb346592e2882d82c28ab5",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "386"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "e954aa43bc3d58a30a967d36b0b0ebf408eea4b1283106d2ca553b0243858d6b",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "mips64le"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "db6ea0cbfcdfe2e7fff3f36b40c2c6ac27933977d71317b30c1905675ec29349",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "ppc64le"},
		Size:      528,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "8f23e10f4610afdde9b856b9367742f1f5ded5c35e2aaa0630d3c5d9ebc2e4cf",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "riscv64"},
		Size:      527,
	}, {
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "069e43a261e5dd787655dbeba5eed96e40f4c9f80f024ecd5d2bd17aab357204",
		},
		MediaType: types.DockerManifestSchema2,
		Platform:  &v1.Platform{OS: "linux", Architecture: "s390x"},
		Size:      528,
	}},
	MediaType:     types.DockerManifestList,
	SchemaVersion: 2,
}

func TestManifestListForImage_RemoteIndex(t *testing.T) {
	t.Parallel()

	type args struct {
		img       string
		platforms []string
	}
	tests := []struct {
		name              string
		args              args
		wantIndexManifest v1.IndexManifest
		wantErr           error
	}{{
		name:    "empty image name",
		args:    args{img: ""},
		wantErr: &name.ErrBadName{},
	}, {
		name:    "invalid image name",
		args:    args{img: "invalid::imagename"},
		wantErr: &name.ErrBadName{},
	}, {
		name:              "valid image name, all platforms",
		args:              args{img: "busybox:1.36.0"},
		wantIndexManifest: busyboxIndexManifest,
	}, {
		name: "valid image name, single platform",
		args: args{img: "busybox:1.36.0", platforms: []string{"linux/amd64"}},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "907ca53d7e2947e849b839b1cd258c98fd3916c60f2e6e70c30edbf741ab6754",
				},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "amd64"},
				Size:      528,
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}, {
		name: "valid image name, multiple platforms",
		args: args{img: "busybox:1.36.0", platforms: []string{"linux/amd64", "linux/riscv64"}},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "907ca53d7e2947e849b839b1cd258c98fd3916c60f2e6e70c30edbf741ab6754",
				},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "amd64"},
				Size:      528,
			}, {
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "8f23e10f4610afdde9b856b9367742f1f5ded5c35e2aaa0630d3c5d9ebc2e4cf",
				},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "riscv64"},
				Size:      527,
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}, {
		name: "valid image name, single platform with variant",
		args: args{img: "busybox:1.36.0", platforms: []string{"linux/arm64/v8"}},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "5e42fbc46b177f10319e8937dd39702e7891ce6d8a42d60c1b4f433f94200bd2",
				},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
				Size:      528,
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}, {
		name: "valid image name, single platform ignoring variant",
		args: args{img: "busybox:1.36.0", platforms: []string{"linux/arm64"}},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest: v1.Hash{
					Algorithm: "sha256",
					Hex:       "5e42fbc46b177f10319e8937dd39702e7891ce6d8a42d60c1b4f433f94200bd2",
				},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
				Size:      528,
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svr := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", string(types.DockerManifestList))
					json.NewEncoder(w).Encode(busyboxIndexManifest)
				}),
			)
			defer svr.Close()

			got, err := ManifestListForImage(
				fmt.Sprintf("%s/%s", svr.Listener.Addr(), tt.args.img),
				tt.args.platforms,
			)
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil {
				require.NotNil(t, got)
				gotIndexManifest, err := got.IndexManifest()
				require.NoError(t, err)
				assert.Equal(t, tt.wantIndexManifest, *gotIndexManifest)
			}
		})
	}
}

var (
	fipsImageManifest = v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestSchema2,
		Config: v1.Descriptor{
			MediaType: types.DockerConfigJSON,
			Size:      1693,
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "a1d425d012a2ff25298c646329404599acfb80ad9db71488ff5d68ef2f7dfa23",
			},
		},
		Layers: []v1.Descriptor{{
			MediaType: types.DockerLayer,
			Size:      804101,
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "b9f88661235d25835ef747dab426861d51c4e9923b92623d422d7ac58eb123e9",
			},
		}, {
			MediaType: types.DockerLayer,
			Size:      670771,
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "cca57b588e6e98eb7820626d44aadefca387e6a83fe0f7a28afd99d021fa23a4",
			},
		}, {
			MediaType: types.DockerLayer,
			Size:      32334806,
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "f98a2fb32546629acb34116e1c5ceb7499cefb3660480a9e702f36579ff31be6",
			},
		}},
	}

	fipsImageConfig = v1.ConfigFile{
		OS:           "linux",
		Architecture: "amd64",
		Variant:      "v1",
		Author:       "Bazel",
		Config: v1.Config{
			Entrypoint: []string{
				"/go-runner",
			},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt",
			},
			Labels: map[string]string{
				"description": "go based runner for distroless scenarios",
				"maintainers": "Kubernetes Authors",
			},
			OnBuild:    nil,
			User:       "0",
			WorkingDir: "/",
		},
		History: []v1.History{{
			Author:    "Bazel",
			CreatedBy: "bazel build ...",
		}, {
			Comment:    "buildkit.dockerfile.v0",
			CreatedBy:  "LABEL maintainers=Kubernetes Authors",
			EmptyLayer: true,
		}, {
			Comment:    "buildkit.dockerfile.v0",
			CreatedBy:  "LABEL description=go based runner for distroless scenarios",
			EmptyLayer: true,
		}, {
			Comment:    "buildkit.dockerfile.v0",
			CreatedBy:  "WORKDIR /",
			EmptyLayer: true,
		}, {
			Comment:   "buildkit.dockerfile.v0",
			CreatedBy: "COPY /workspace/go-runner . # buildkit",
		}, {
			Comment:    "buildkit.dockerfile.v0",
			CreatedBy:  "ENTRYPOINT [\"/go-runner\"]",
			EmptyLayer: true,
		}, {
			Comment:    "buildkit.dockerfile.v0",
			CreatedBy:  "ARG BINARY",
			EmptyLayer: true,
		}, {
			Comment:   "buildkit.dockerfile.v0",
			CreatedBy: "COPY /kube-apiserver /usr/local/bin/kube-apiserver # buildkit",
		}},
		RootFS: v1.RootFS{
			DiffIDs: []v1.Hash{
				{
					Algorithm: "sha256",
					Hex:       "8d7366c22fd8219bfcfb61ed28457854c80e310b0d736b67861b2ea7fcd77843",
				},
				{
					Algorithm: "sha256",
					Hex:       "501a749ae1d5e55c85bfa8c4e0ea6876d909c3fdd70503f55345bc441a44352d",
				},
				{
					Algorithm: "sha256",
					Hex:       "69f6bd8c0a4a191dc624534174a6c0b86a6ac75ae886fb2b68b7c9e494286b09",
				},
			},
			Type: "layers",
		},
	}
)

func TestManifestListForImage_RemoteImage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(fipsImageManifest)
	require.NoError(t, err)
	sizeFIPSImageManifest := buf.Len()
	h := sha256.New()
	h.Write(buf.Bytes())
	digestFIPSImageManifest := hex.EncodeToString(h.Sum(nil))

	type args struct {
		img       string
		platforms []string
	}
	tests := []struct {
		name              string
		args              args
		wantIndexManifest v1.IndexManifest
		wantErr           string
	}{{
		name: "valid image name, all platforms",
		args: args{img: "mesosphere/kube-apiserver:v1.24.4_fips.0"},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest:    v1.Hash{Algorithm: "sha256", Hex: digestFIPSImageManifest},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v1"},
				Size:      int64(sizeFIPSImageManifest),
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}, {
		name: "valid image name, multiple platforms",
		args: args{
			img:       "mesosphere/kube-apiserver:v1.24.4_fips.0",
			platforms: []string{"linux/amd64", "linux/riscv64"},
		},
		wantErr: "is a single platform image, cannot create an index with multiple platforms",
	}, {
		name: "valid image name, single platform with variant",
		args: args{
			img:       "mesosphere/kube-apiserver:v1.24.4_fips.0",
			platforms: []string{"linux/amd64/v1"},
		},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest:    v1.Hash{Algorithm: "sha256", Hex: digestFIPSImageManifest},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v1"},
				Size:      int64(sizeFIPSImageManifest),
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}, {
		name: "valid image name, single platform ignoring variant",
		args: args{
			img:       "mesosphere/kube-apiserver:v1.24.4_fips.0",
			platforms: []string{"linux/amd64"},
		},
		wantIndexManifest: v1.IndexManifest{
			Manifests: []v1.Descriptor{{
				Digest:    v1.Hash{Algorithm: "sha256", Hex: digestFIPSImageManifest},
				MediaType: types.DockerManifestSchema2,
				Platform:  &v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v1"},
				Size:      int64(sizeFIPSImageManifest),
			}},
			MediaType:     types.DockerManifestList,
			SchemaVersion: 2,
		},
	}}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			mux.Handle("/v2/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			mux.Handle(
				"/v2/mesosphere/kube-apiserver/manifests/v1.24.4_fips.0",
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", string(types.DockerManifestSchema2))
					json.NewEncoder(w).Encode(fipsImageManifest)
				}),
			)
			mux.Handle(
				"/v2/mesosphere/kube-apiserver/blobs/sha256:a1d425d012a2ff25298c646329404599acfb80ad9db71488ff5d68ef2f7dfa23",
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", string(types.DockerConfigJSON))
					json.NewEncoder(w).Encode(fipsImageConfig)
				}),
			)
			svr := httptest.NewServer(mux)
			defer svr.Close()

			got, err := ManifestListForImage(
				fmt.Sprintf("%s/%s", svr.Listener.Addr(), tt.args.img),
				tt.args.platforms,
			)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				gotIndexManifest, err := got.IndexManifest()
				require.NoError(t, err)
				assert.Equal(t, tt.wantIndexManifest, *gotIndexManifest)
			}
		})
	}
}
