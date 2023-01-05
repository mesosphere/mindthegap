// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"path/filepath"
	"testing"

	"github.com/mesosphere/mindthegap/test/e2e/imagebundle/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var artifacts helpers.Artifacts

func TestImagebundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Bundle Suite", Label("image", "imagebundle"))
}

var _ = BeforeSuite(func() {
	var err error
	artifacts, err = helpers.ParseArtifactsFile(filepath.Join("..",
		"..",
		"..",
		"dist", "artifacts.json"),
	)
	Expect(err).NotTo(HaveOccurred())
})
