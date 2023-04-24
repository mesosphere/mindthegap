// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helmbundle_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHelmBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helm Bundle Suite", Label("helm", "helmbundle"))
}
