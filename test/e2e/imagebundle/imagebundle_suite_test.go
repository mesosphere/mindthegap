// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestImagebundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Bundle Suite", Label("image", "imagebundle"))
}
