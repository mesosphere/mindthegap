// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"fmt"
	"regexp"
)

// regular expression to represent all ECR endpoints. see list at https://docs.aws.amazon.com/general/latest/gr/ecr.html
var ecrRegistryRegexp = regexp.MustCompile(
	`^(?:https://)?([a-zA-Z0-9]+)\.dkr\.ecr(-fips)?\.([^.]+)\.amazonaws\.com/?`,
)

func IsECRRegistry(registryAddress string) bool {
	return ecrRegistryRegexp.MatchString(registryAddress)
}

func ParseECRRegistry(
	registryAddress string,
) (accountID string, fips bool, region string, err error) {
	matches := ecrRegistryRegexp.FindStringSubmatch(registryAddress)
	if len(matches) == 0 {
		return "", false, "", fmt.Errorf(
			"only private Amazon Elastic Container Registry supported")
	} else if len(matches) < 3 {
		return "", false, "", fmt.Errorf(
			"%q is not a valid repository URI for private Amazon Elastic Container Registry", registryAddress)
	}

	accountID = matches[1]
	fips = (matches[2] == "-fips")
	region = matches[3]
	err = nil
	return accountID, fips, region, err
}
