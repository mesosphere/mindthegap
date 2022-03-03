// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package skopeo

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containers/skopeo/cmd/skopeo/inspect"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest/manifestlist"
	"github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/docker/cli/cli/config"
	"k8s.io/klog/v2"
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

func OS(opsys string) SkopeoOption {
	return func() string {
		return "--override-os=" + opsys
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
}

type CleanupFunc func() error

func NewRunner() (*Runner, CleanupFunc) {
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
	//nolint:gosec // Binary must be executable.
	if err = os.WriteFile(r.unpackedSkopeoPath, skopeoBinary, 0o700); err != nil {
		panic(err)
	}
	r.unpackedSkopeoPolicyPath = filepath.Join(tempDir, "policy.json")
	if err = os.WriteFile(r.unpackedSkopeoPolicyPath, defaultSkopeoPolicy, 0o400); err != nil {
		panic(err)
	}
}

// Manifest defines a schema2 manifest.
type Manifest struct {
	schema2.Manifest

	// Annotations holds image manifest annotations.
	Annotations map[string]string `json:"annotations"`
}

func (r *Runner) Copy(
	ctx context.Context,
	src, dest string,
	opts ...SkopeoOption,
) (stdout, stderr []byte, err error) {
	r.unpacked.Do(r.mustUnpack)

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
) (manifests manifestlist.ManifestList, stdout, stderr []byte, err error) {
	inspectArgs := []string{
		"inspect",
		"--raw",
		imageName,
	}

	rawStdout, rawStderr, err := r.run(ctx, inspectArgs, opts...)
	if err != nil {
		return manifestlist.ManifestList{}, rawStdout, rawStderr, fmt.Errorf(
			"failed to read image manifest: %w",
			err,
		)
	}

	var ml manifestlist.ManifestList
	dec := json.NewDecoder(bytes.NewReader(rawStdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ml); err == nil {
		return ml, rawStdout, rawStderr, nil
	}

	var m Manifest
	dec = json.NewDecoder(bytes.NewReader(rawStdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return manifestlist.ManifestList{}, rawStdout, rawStderr, fmt.Errorf(
			"failed to deserialize manifest: %w",
			err,
		)
	}

	inspectArgs = []string{
		"inspect",
		imageName,
	}

	inspectStdout, inspectStderr, err := r.run(ctx, inspectArgs, opts...)
	if err != nil {
		return manifestlist.ManifestList{}, inspectStdout, inspectStderr, fmt.Errorf(
			"failed to read image manifest: %w",
			err,
		)
	}

	var i inspect.Output
	dec = json.NewDecoder(bytes.NewReader(inspectStdout))
	if err := dec.Decode(&i); err != nil {
		return manifestlist.ManifestList{}, inspectStdout, inspectStderr,
			fmt.Errorf("failed to deserialize manifest: %w", err)
	}

	ml = manifestlist.ManifestList{
		Versioned: manifestlist.SchemaVersion,
		Manifests: []manifestlist.ManifestDescriptor{{
			Descriptor: distribution.Descriptor{
				MediaType: schema2.MediaTypeManifest,
				Digest:    i.Digest,
				Size:      int64(len(rawStdout)),
			},
			Platform: manifestlist.PlatformSpec{
				OS:           i.Os,
				Architecture: i.Architecture,
			},
		}},
	}

	return ml, rawStdout, rawStderr, nil
}

func (r *Runner) CopyManifest(
	ctx context.Context, manifest manifestlist.ManifestList, dest string, opts ...SkopeoOption,
) (stdout, stderr []byte, err error) {
	td, err := os.MkdirTemp("", ".image-bundle-manifest-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(td)

	mf, err := os.Create(filepath.Join(td, "manifest.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create manifest.jsin file: %w", err)
	}
	defer mf.Close()

	// No need to check error - safe to enscode this type. See https://github.com/breml/errchkjson#safe
	// for details
	_ = json.NewEncoder(mf).Encode(manifest)

	return r.Copy(
		ctx,
		"dir:"+td,
		dest,
		append(opts, func() string { return "--multi-arch=index-only" })...)
}

func (r *Runner) run(
	ctx context.Context,
	baseArgs []string,
	opts ...SkopeoOption,
) (stdout, stderr []byte, err error) {
	r.unpacked.Do(r.mustUnpack)

	skopeoArgs := make([]string, 0, len(baseArgs)+len(opts))
	skopeoArgs = append(skopeoArgs, baseArgs...)

	for _, o := range opts {
		skopeoArgs = append(skopeoArgs, o())
	}

	if klog.V(4).Enabled() {
		skopeoArgs = append(skopeoArgs, Debug()())
	}

	klog.V(4).Info("Running skopeo", append([]string{r.unpackedSkopeoPath}, skopeoArgs...))

	//nolint:gosec // Args are valid
	cmd := exec.CommandContext(ctx, r.unpackedSkopeoPath, skopeoArgs...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, err = cmd.Output()
	return stdout, stderrBuf.Bytes(), err
}

func (r *Runner) AttemptToLoginToRegistry(
	ctx context.Context,
	registryName string,
) (stdout, stderr []byte, err error) {
	var skopeoOpts []SkopeoOption
	getLoginStdout, getLoginStderr, err := r.run(
		ctx,
		[]string{"login", "--get-login", registryName},
		skopeoOpts...)
	if err == nil {
		return getLoginStdout, getLoginStderr, nil
	}
	if err != nil &&
		!strings.Contains(string(getLoginStderr), fmt.Sprintf("not logged into %s", registryName)) {
		return getLoginStdout, getLoginStderr, fmt.Errorf(
			"failed to check if already logged in to %s: %w",
			registryName,
			err,
		)
	}

	stdoutBuf := bytes.NewBuffer(getLoginStdout)
	stderrBuf := bytes.NewBuffer(getLoginStderr)

	configFile := config.LoadDefaultConfigFile(stderrBuf)

	registryNamesToTry := []string{registryName}
	if registryName == "docker.io" {
		registryNamesToTry = append(registryNamesToTry, "https://index.docker.io/v1/")
	}

	for _, reg := range registryNamesToTry {
		authConfig, err := configFile.GetAuthConfig(reg)
		if err != nil {
			return getLoginStdout, getLoginStderr, fmt.Errorf(
				"failed to get auth config for %s: %w",
				registryName,
				err,
			)
		}
		username := authConfig.Username
		password := authConfig.Password

		if authConfig.Auth != "" {
			c, err := base64.StdEncoding.DecodeString(authConfig.Auth)
			if err != nil {
				return getLoginStdout, getLoginStderr, fmt.Errorf(
					"failed to read credentials from Docker config file: %w",
					err,
				)
			}
			cs := string(c)
			s := strings.IndexByte(cs, ':')
			if s >= 0 {
				username = cs[:s]
				password = cs[s+1:]
			}
		}

		if username != "" && password != "" {
			loginStdout, loginStderr, err := r.run(
				ctx,
				[]string{
					"login",
					registryName,
					"--username",
					username,
					"--password",
					password,
				},
				skopeoOpts...,
			)
			_, _ = stdoutBuf.Write(loginStdout)
			_, _ = stderrBuf.Write(loginStderr)
			if err == nil {
				return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
			}
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf(
				"failed to login to %s: %w",
				registryName,
				err,
			)
		}
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}
