// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/utils/strings/slices"
)

func ValidateFlagsThatRequireValues(cmd *cobra.Command, requiredFlagsWithValues ...string) error {
	fs := cmd.Flags()

	missingFlagValues := []string{}
	for _, flagName := range requiredFlagsWithValues {
		foundFlag := fs.Lookup(flagName)
		if foundFlag == nil {
			continue
		}

		if sv, ok := foundFlag.Value.(pflag.SliceValue); ok {
			if len(slices.Filter(nil, sv.GetSlice(), func(s string) bool { return s != "" })) == 0 {
				missingFlagValues = append(missingFlagValues, flagName)
			}
		} else {
			if foundFlag.Value.String() == "" {
				missingFlagValues = append(missingFlagValues, flagName)
			}
		}
	}

	if len(missingFlagValues) > 0 {
		return fmt.Errorf(
			`the following flags require value(s) to be specified: "%s"`,
			strings.Join(missingFlagValues, `", "`),
		)
	}
	return nil
}
