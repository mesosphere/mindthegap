// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func ValidateRequiredFlagValues(cmd *cobra.Command, requiredFlagsWithValues ...string) error {
	fs := cmd.Flags()

	missingFlagValues := []string{}
	for _, flagName := range requiredFlagsWithValues {
		foundFlag := fs.Lookup(flagName)
		if foundFlag == nil {
			continue
		}

		if foundFlag.Value.String() == "" || foundFlag.Value.String() == "[]" {
			missingFlagValues = append(missingFlagValues, flagName)
		}
	}

	if len(missingFlagValues) > 0 {
		return fmt.Errorf(`required flag value(s) "%s" not set`, strings.Join(missingFlagValues, `", "`))
	}
	return nil
}
