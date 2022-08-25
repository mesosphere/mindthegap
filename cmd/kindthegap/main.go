// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"math/rand"
	"time"

	"github.com/mesosphere/mindthegap/cmd/kindthegap/root"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	root.Execute()
}
