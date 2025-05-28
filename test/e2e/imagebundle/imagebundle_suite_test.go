// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mesosphere/mindthegap/test/e2e/helpers"
)

var artifacts helpers.Artifacts

func TestImagebundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Bundle Suite", Label("image", "imagebundle"))
}

var _ = BeforeSuite(func() {
	var err error
	artifactsFileAbs, err := filepath.Abs(filepath.Join("..",
		"..",
		"..",
		"dist", "artifacts.json"))
	Expect(err).NotTo(HaveOccurred())
	relArtifacts, err := helpers.ParseArtifactsFile(artifactsFileAbs)
	Expect(err).NotTo(HaveOccurred())

	artifacts = make(helpers.Artifacts, 0, len(relArtifacts))
	for _, a := range relArtifacts {
		if a.Path != "" {
			a.Path = filepath.Join(filepath.Dir(artifactsFileAbs), "..", a.Path)
		}
		artifacts = append(artifacts, a)
	}
})
