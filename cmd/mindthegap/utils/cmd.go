// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import "github.com/spf13/cobra"

func AddCmdAnnotation(cmd *cobra.Command, key, value string) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string, 1)
	}
	cmd.Annotations[key] = value
}
