// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	"github.com/mesosphere/dkp-cli-runtime/core/cmd/version"
)

func Useragent() string {
	return fmt.Sprintf("mindthegap/%s", version.GetVersion().GitVersion)
}
