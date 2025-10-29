// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
)

func TestNewPushBundleOpts(t *testing.T) {
	t.Run("success with required fields", func(t *testing.T) {
		bundleFiles := []string{"bundle1.tar", "bundle2.tar"}
		registryURI := &flags.RegistryURI{}
		err := registryURI.Set("registry.example.com/path")
		require.NoError(t, err)

		cfg, err := NewPushBundleOpts(bundleFiles, registryURI)

		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, bundleFiles, cfg.bundleFiles)
		assert.Equal(t, registryURI, cfg.registryURI)
		assert.Equal(t, Overwrite, cfg.onExistingTag) // default value
		assert.Equal(t, 1, cfg.imagePushConcurrency)  // default value
	})

	t.Run("fails with no bundle files", func(t *testing.T) {
		registryURI := &flags.RegistryURI{}
		err := registryURI.Set("registry.example.com")
		require.NoError(t, err)

		cfg, err := NewPushBundleOpts([]string{}, registryURI)

		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "at least one bundle file is required")
	})

	t.Run("fails with empty registry URI", func(t *testing.T) {
		bundleFiles := []string{"bundle.tar"}
		registryURI := &flags.RegistryURI{}

		cfg, err := NewPushBundleOpts(bundleFiles, registryURI)

		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "registry URI is required")
	})
}

func TestPushBundleOpts_WithRegistryCredentials(t *testing.T) {
	cfg := &pushBundleOpts{}

	t.Run("success with both username and password", func(t *testing.T) {
		err := cfg.WithRegistryCredentials("user", "pass")

		require.NoError(t, err)
		assert.Equal(t, "user", cfg.registryUsername)
		assert.Equal(t, "pass", cfg.registryPassword)
	})

	t.Run("success with both empty", func(t *testing.T) {
		err := cfg.WithRegistryCredentials("", "")

		require.NoError(t, err)
		assert.Empty(t, cfg.registryUsername)
		assert.Empty(t, cfg.registryPassword)
	})

	t.Run("fails with only username", func(t *testing.T) {
		err := cfg.WithRegistryCredentials("user", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "both username and password must be provided together")
	})

	t.Run("fails with only password", func(t *testing.T) {
		err := cfg.WithRegistryCredentials("", "pass")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "both username and password must be provided together")
	})
}

func TestPushBundleOpts_WithRegistryCACertificateFile(t *testing.T) {
	t.Run("success setting CA cert file", func(t *testing.T) {
		cfg := &pushBundleOpts{}
		err := cfg.WithRegistryCACertificateFile("/path/to/ca.crt")

		require.NoError(t, err)
		assert.Equal(t, "/path/to/ca.crt", cfg.registryCACertificateFile)
	})

	t.Run("fails when skip TLS verify is already set", func(t *testing.T) {
		cfg := &pushBundleOpts{registrySkipTLSVerify: true}
		err := cfg.WithRegistryCACertificateFile("/path/to/ca.crt")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both CA certificate and skip TLS verify")
	})

	t.Run("success with empty cert file", func(t *testing.T) {
		cfg := &pushBundleOpts{}
		err := cfg.WithRegistryCACertificateFile("")

		require.NoError(t, err)
		assert.Empty(t, cfg.registryCACertificateFile)
	})
}

func TestPushBundleOpts_WithRegistrySkipTLSVerify(t *testing.T) {
	t.Run("success setting skip TLS verify to true", func(t *testing.T) {
		cfg := &pushBundleOpts{}
		err := cfg.WithRegistrySkipTLSVerify(true)

		require.NoError(t, err)
		assert.True(t, cfg.registrySkipTLSVerify)
	})

	t.Run("success setting skip TLS verify to false", func(t *testing.T) {
		cfg := &pushBundleOpts{}
		err := cfg.WithRegistrySkipTLSVerify(false)

		require.NoError(t, err)
		assert.False(t, cfg.registrySkipTLSVerify)
	})

	t.Run("fails when CA cert file is already set", func(t *testing.T) {
		cfg := &pushBundleOpts{registryCACertificateFile: "/path/to/ca.crt"}
		err := cfg.WithRegistrySkipTLSVerify(true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both CA certificate and skip TLS verify")
	})

	t.Run("success setting skip to false when CA cert is set", func(t *testing.T) {
		cfg := &pushBundleOpts{registryCACertificateFile: "/path/to/ca.crt"}
		err := cfg.WithRegistrySkipTLSVerify(false)

		require.NoError(t, err)
		assert.False(t, cfg.registrySkipTLSVerify)
	})
}

func TestPushBundleOpts_WithChaining(t *testing.T) {
	bundleFiles := []string{"bundle.tar"}
	registryURI := &flags.RegistryURI{}
	err := registryURI.Set("registry.example.com")
	require.NoError(t, err)

	cfg, err := NewPushBundleOpts(bundleFiles, registryURI)
	require.NoError(t, err)

	// Test method chaining for methods that return *pushBundleOpts
	cfg.WithECRLifecyclePolicy("policy.json").
		WithOnExistingTag(Skip).
		WithImagePushConcurrency(5).
		WithForceOCIMediaTypes(true)

	assert.Equal(t, "policy.json", cfg.ecrLifecyclePolicy)
	assert.Equal(t, Skip, cfg.onExistingTag)
	assert.Equal(t, 5, cfg.imagePushConcurrency)
	assert.True(t, cfg.forceOCIMediaTypes)
}
