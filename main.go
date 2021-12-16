// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/mesosphere/mindthegap/cmd"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	logrus.SetOutput(io.Discard)

	cmd.Execute()
}
