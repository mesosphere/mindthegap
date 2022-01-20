// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package skopeo

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/distribution/distribution/v3/manifest/manifestlist"
	"github.com/docker/cli/cli/config"
	"github.com/mesosphere/dkp-cli-runtime/core/output"
)

//go:embed default-policy.json
var defaultSkopeoPolicy []byte

type SkopeoOption func() string

func DisableSrcTLSVerify() SkopeoOption {
	return func() string {
		return "--src-tls-verify=false"
	}
}

func DisableDestTLSVerify() SkopeoOption {
	return func() string {
		return "--dest-tls-verify=false"
	}
}

func AllImages() SkopeoOption {
	return func() string {
		return "--all"
	}
}

func OS(os string) SkopeoOption {
	return func() string {
		return "--override-os=" + os
	}
}

func Arch(arch string) SkopeoOption {
	return func() string {
		return "--override-arch=" + arch
	}
}

func Variant(variant string) SkopeoOption {
	return func() string {
		return "--override-variant=" + variant
	}
}

func Debug() SkopeoOption {
	return func() string {
		return "--debug"
	}
}

func All() SkopeoOption {
	return func() string {
		return "--all"
	}
}

func SrcCredentials(username, password string) SkopeoOption {
	return func() string {
		return fmt.Sprintf("--src-creds=%s:%s", username, password)
	}
}

func DestCredentials(username, password string) SkopeoOption {
	return func() string {
		return fmt.Sprintf("--dest-creds=%s:%s", username, password)
	}
}

type Runner struct {
	unpacked                 sync.Once
	unpackedSkopeoPath       string
	unpackedSkopeoPolicyPath string
	out                      output.Output
}

type CleanupFunc func() error

func NewRunner(out output.Output) (*Runner, CleanupFunc) {
	r := &Runner{}
	return r, func() error {
		return os.RemoveAll(filepath.Dir(r.unpackedSkopeoPath))
	}
}

func (r *Runner) mustUnpack() {
	tempDir, err := os.MkdirTemp("", "skopeo-*")
	if err != nil {
		panic(err)
	}
	r.unpackedSkopeoPath = filepath.Join(tempDir, "skopeo")
	//nolint:gosec // Binary must be executable
	if err = os.WriteFile(r.unpackedSkopeoPath, skopeoBinary, 0700); err != nil {
		panic(err)
	}
	r.unpackedSkopeoPolicyPath = filepath.Join(tempDir, "policy.json")
	if err = os.WriteFile(r.unpackedSkopeoPolicyPath, defaultSkopeoPolicy, 0400); err != nil {
		panic(err)
	}
}

func (r *Runner) Copy(ctx context.Context, src, dest string, opts ...SkopeoOption) ([]byte, error) {
	copyArgs := []string{
		"copy",
		"--policy", r.unpackedSkopeoPolicyPath,
		"--preserve-digests",
		src,
		dest,
	}

	return r.run(ctx, copyArgs, opts...)
}

func (r *Runner) InspectManifest(
	ctx context.Context, imageName string, opts ...SkopeoOption,
) (manifestlist.ManifestList, []byte, error) {
	inspectArgs := []string{
		"inspect",
		"--raw",
		imageName,
	}

	output, err := r.run(ctx, inspectArgs, opts...)
	if err != nil {
		return manifestlist.ManifestList{}, output, fmt.Errorf("failed to read image manifest: %w", err)
	}
	var m manifestlist.ManifestList
	dec := json.NewDecoder(bytes.NewReader(output))
	if err := dec.Decode(&m); err != nil {
		return manifestlist.ManifestList{}, output, fmt.Errorf("failed to deserialize manifest: %w", err)
	}

	return m, output, nil
}

func (r *Runner) CopyManifest(
	ctx context.Context, manifest manifestlist.ManifestList, dest string, opts ...SkopeoOption,
) ([]byte, error) {
	td, err := os.MkdirTemp("", ".image-bundle-manifest-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(td)

	mf, err := os.Create(filepath.Join(td, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest.jsin file: %w", err)
	}
	defer mf.Close()

	if err := json.NewEncoder(mf).Encode(manifest); err != nil {
		return nil, fmt.Errorf("failed to encode manifest: %w", err)
	}

	return r.Copy(ctx, "dir:"+td, dest, append(opts, func() string { return "--multi-arch=index-only" })...)
}

func (r *Runner) run(ctx context.Context, baseArgs []string, opts ...SkopeoOption) ([]byte, error) {
	r.unpacked.Do(r.mustUnpack)

	skopeoArgs := make([]string, 0, len(baseArgs)+len(opts))
	skopeoArgs = append(skopeoArgs, baseArgs...)

	for _, o := range opts {
		skopeoArgs = append(skopeoArgs, o())
	}

	r.out.V(4).Infof("Running skopeo: %s %v", r.unpackedSkopeoPath, skopeoArgs)
	//nolint:gosec // Args are valid
	cmd := exec.CommandContext(ctx, r.unpackedSkopeoPath, skopeoArgs...)
	return cmd.CombinedOutput()
}

func (r *Runner) AttemptToLoginToRegistry(ctx context.Context, registryName string) error {
	var skopeoOpts []SkopeoOption
	getLoginOutput, err := r.run(ctx, []string{"login", "--get-login", registryName}, skopeoOpts...)
	if err == nil {
		r.out.V(4).Info(string(getLoginOutput))
		return nil
	}
	if err != nil && !strings.Contains(string(getLoginOutput), fmt.Sprintf("not logged into %s", registryName)) {
		r.out.Info(string(getLoginOutput))
		return fmt.Errorf("failed to check if already logged in to %s: %w", registryName, err)
	}

	configFile := config.LoadDefaultConfigFile(io.Discard)

	registryNamesToTry := []string{registryName}
	if registryName == "docker.io" {
		registryNamesToTry = append(registryNamesToTry, "https://index.docker.io/v1/")
	}

	for _, reg := range registryNamesToTry {
		authConfig, err := configFile.GetAuthConfig(reg)
		if err != nil {
			return fmt.Errorf("failed to get auth config for %s: %w", registryName, err)
		}
		if authConfig.Username != "" && authConfig.Password != "" {
			loginOutput, err := r.run(ctx,
				[]string{"login", registryName, "--username", authConfig.Username, "--password", authConfig.Password},
				skopeoOpts...,
			)
			if err == nil {
				r.out.V(4).Info(string(loginOutput))
				return nil
			}
			r.out.Info(string(loginOutput))
			return fmt.Errorf("failed to login to %s: %w", registryName, err)
		}
	}

	return nil
}
