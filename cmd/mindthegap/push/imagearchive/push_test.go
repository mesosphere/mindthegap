// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagearchive_test

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
	"github.com/mesosphere/mindthegap/images/archive/testutil"
)

func TestPushDockerArchive_EndToEnd(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	regHost := srv.Listener.Addr().String()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "src.tar")
	img := testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := imagearchive.NewCommand(out)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--image-archive", archivePath,
		"--to-registry", fmt.Sprintf("http://%s", regHost),
		"--to-registry-insecure-skip-tls-verify",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
	}

	pulled, err := crane.Pull(fmt.Sprintf("%s/app:v1", regHost))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	gotDigest, err := pulled.Digest()
	if err != nil {
		t.Fatalf("got digest: %v", err)
	}
	wantDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("want digest: %v", err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("digest mismatch: got %s, want %s", gotDigest, wantDigest)
	}
}

func basicAuthWrap(inner http.Handler, user, pass string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok := r.BasicAuth()
		if !ok || gotUser != user || gotPass != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

func TestPushDockerArchive_BasicAuth(t *testing.T) {
	const user, pass = "u", "p"
	srv := httptest.NewServer(basicAuthWrap(registry.New(), user, pass))
	defer srv.Close()
	regHost := srv.Listener.Addr().String()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "auth.tar")
	testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := imagearchive.NewCommand(out)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--image-archive", archivePath,
		"--to-registry", fmt.Sprintf("http://%s", regHost),
		"--to-registry-insecure-skip-tls-verify",
		"--to-registry-username", user,
		"--to-registry-password", pass,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
	}
}

// bearerAuthWrap wraps inner with a Bearer-token challenge served from the
// same host:port as the registry. /token mints a token in exchange for
// HTTP basic credentials; every other request requires a matching bearer
// token. The realm advertised in the WWW-Authenticate header points back
// at the same httptest server (a loopback IP), which is exactly the
// configuration that go-containerregistry's validateRealmURL would reject
// before v0.21.6 — see google/go-containerregistry#2258 / #2302.
func bearerAuthWrap(inner http.Handler, user, pass, token string, realmHost *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			gotUser, gotPass, ok := r.BasicAuth()
			if !ok || gotUser != user || gotPass != pass {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"token":%q,"access_token":%q}`, token, token)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.Header().Set(
				"WWW-Authenticate",
				fmt.Sprintf(`Bearer realm="http://%s/token",service="registry"`, *realmHost),
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

// TestPushDockerArchive_BearerAuthSameHostLoopbackRealm is a regression test
// for google/go-containerregistry#2258 (fixed by #2302, available in
// v0.21.6). A registry running on a loopback address that serves its own
// bearer-token endpoint (realm host == registry host == 127.0.0.1) must
// not be rejected by validateRealmURL's private/link-local IP block. This
// is the configuration that broke `mindthegap push bundle` against
// on-prem Harbor at https://10.162.182.23:5000/library — see NCN-114223.
func TestPushDockerArchive_BearerAuthSameHostLoopbackRealm(t *testing.T) {
	const user, pass, token = "u", "p", "test-token"
	// realmHost is filled in once we know the listener address, then
	// captured by the handler closure via pointer.
	var realmHost string
	srv := httptest.NewServer(bearerAuthWrap(registry.New(), user, pass, token, &realmHost))
	defer srv.Close()
	regHost := srv.Listener.Addr().String()
	realmHost = regHost

	// Sanity-check that httptest bound to a loopback IP literal — if it
	// ever switches to a hostname, this test stops exercising the
	// private-IP path that v0.21.6 had to special-case.
	host, _, err := net.SplitHostPort(regHost)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", regHost, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		t.Fatalf("expected httptest to bind to a loopback IP literal, got %q", host)
	}

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "bearer.tar")
	testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := imagearchive.NewCommand(out)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--image-archive", archivePath,
		"--to-registry", fmt.Sprintf("http://%s", regHost),
		"--to-registry-insecure-skip-tls-verify",
		"--to-registry-username", user,
		"--to-registry-password", pass,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
	}
}

func TestPushDockerArchive_BasicAuthWrongPassword(t *testing.T) {
	const user, pass = "u", "p"
	srv := httptest.NewServer(basicAuthWrap(registry.New(), user, pass))
	defer srv.Close()
	regHost := srv.Listener.Addr().String()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "auth-fail.tar")
	testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := imagearchive.NewCommand(out)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--image-archive", archivePath,
		"--to-registry", fmt.Sprintf("http://%s", regHost),
		"--to-registry-insecure-skip-tls-verify",
		"--to-registry-username", user,
		"--to-registry-password", "wrong",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected authentication error, got nil")
	}
}

func TestPush_TaglessWithoutOverride(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	regHost := srv.Listener.Addr().String()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "tagless.tar")
	testutil.BuildOCIArchive(t, archivePath, "")

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := imagearchive.NewCommand(out)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--image-archive", archivePath,
		"--to-registry", fmt.Sprintf("http://%s", regHost),
		"--to-registry-insecure-skip-tls-verify",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--image-tag") {
		t.Fatalf("error does not mention --image-tag: %v", err)
	}
}

func TestPushOCIArchive_EndToEnd(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	regHost := srv.Listener.Addr().String()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "src.tar")
	img := testutil.BuildOCIArchive(t, archivePath, "example.com/app:v1")

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := imagearchive.NewCommand(out)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--image-archive", archivePath,
		"--to-registry", fmt.Sprintf("http://%s", regHost),
		"--to-registry-insecure-skip-tls-verify",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
	}

	pulled, err := crane.Pull(fmt.Sprintf("%s/app:v1", regHost))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	gotDigest, err := pulled.Digest()
	if err != nil {
		t.Fatalf("got digest: %v", err)
	}
	wantDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("want digest: %v", err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("digest mismatch: got %s, want %s", gotDigest, wantDigest)
	}
}
