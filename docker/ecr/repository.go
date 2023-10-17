// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/utils/ptr"
)

func ClientForRegistry(registryAddress string) (*ecr.Client, error) {
	_, _, region, err := ParseECRRegistry(registryAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ECR registry host URI: %w", err)
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}

	// Using the Config value, create the ECR client
	return ecr.NewFromConfig(cfg), nil
}

func EnsureRepositoryExistsFunc(ecrClient *ecr.Client, ecrLifecyclePolicy string) func(
	destRepositoryName name.Repository, _ ...string,
) error {
	return func(
		destRepositoryName name.Repository, _ ...string,
	) error {
		_, repositoryName, _ := strings.Cut(destRepositoryName.Name(), "/")

		repos, err := ecrClient.DescribeRepositories(
			context.TODO(),
			&ecr.DescribeRepositoriesInput{
				RepositoryNames: []string{repositoryName},
			},
		)
		repoNotExistsErr := &types.RepositoryNotFoundException{}
		if err != nil && !errors.As(err, &repoNotExistsErr) {
			return fmt.Errorf("failed to check if ECR repository exists: %w", err)
		}
		if repos != nil && len(repos.Repositories) > 0 {
			return nil
		}

		_, err = ecrClient.CreateRepository(
			context.TODO(),
			&ecr.CreateRepositoryInput{
				RepositoryName:             &repositoryName,
				ImageScanningConfiguration: &types.ImageScanningConfiguration{ScanOnPush: true},
			},
		)
		if err != nil {
			return fmt.Errorf("failed to create reposiotry in ECR: %w", err)
		}

		if ecrLifecyclePolicy == "" {
			return nil
		}
		ecrLifecyclePolicyText, err := os.ReadFile(ecrLifecyclePolicy)
		if err != nil {
			return fmt.Errorf(
				"failed to read ECR lifecycle policy from %q: %w",
				ecrLifecyclePolicy,
				err,
			)
		}
		_, err = ecrClient.PutLifecyclePolicy(
			context.TODO(),
			&ecr.PutLifecyclePolicyInput{
				RepositoryName:      &repositoryName,
				LifecyclePolicyText: ptr.To(string(ecrLifecyclePolicyText)),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to apply ECR repository lifecycle policy: %w", err)
		}

		return nil
	}
}

func RetrieveUsernameAndToken(ecrClient *ecr.Client) (username, token string, err error) {
	// Passing nil as second parameter as passing registry ID is deprecated and does not affect authorization.
	out, err := ecrClient.GetAuthorizationToken(context.Background(), nil)
	if err != nil {
		return "", "", err
	}
	// Returned token is a base64-encoded `<username>:<password>``. Username will normally be AWS but that is not
	// guaranteed.
	base64EncodedAuthorizationToken := aws.ToString(out.AuthorizationData[0].AuthorizationToken)

	decodedAuthorizationToken, err := base64.StdEncoding.DecodeString(
		base64EncodedAuthorizationToken,
	)
	if err != nil {
		return "", "", err
	}
	username, token, _ = strings.Cut(string(decodedAuthorizationToken), ":")
	return username, token, nil
}
