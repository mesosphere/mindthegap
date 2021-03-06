// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"k8s.io/utils/pointer"
)

func EnsureRepositoryExistsFunc(ecrLifecyclePolicy string) func(
	_, imageName string, _ ...string,
) error {
	return func(
		_, imageName string, _ ...string,
	) error {
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
		if err != nil {
			log.Fatalf("unable to load SDK config, %v", err)
		}

		// Using the Config value, create the S3 client
		svc := ecr.NewFromConfig(cfg)
		repos, err := svc.DescribeRepositories(
			context.TODO(),
			&ecr.DescribeRepositoriesInput{
				RepositoryNames: []string{imageName},
			},
		)
		repoNotExistsErr := &types.RepositoryNotFoundException{}
		if err != nil && !errors.As(err, &repoNotExistsErr) {
			return fmt.Errorf("failed to check if ECR repository exists: %w", err)
		}
		if repos != nil && len(repos.Repositories) > 0 {
			return nil
		}

		_, err = svc.CreateRepository(
			context.TODO(),
			&ecr.CreateRepositoryInput{
				RepositoryName:             &imageName,
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
		_, err = svc.PutLifecyclePolicy(
			context.TODO(),
			&ecr.PutLifecyclePolicyInput{
				RepositoryName:      &imageName,
				LifecyclePolicyText: pointer.String(string(ecrLifecyclePolicyText)),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to apply ECR repository lifecycle policy: %w", err)
		}

		return nil
	}
}
