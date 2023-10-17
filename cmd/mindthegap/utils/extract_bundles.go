// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/config"
)

func ExtractBundles(
	dest string,
	out output.Output,
	imageBundleFiles ...string,
) (*config.ImagesConfig, *config.HelmChartsConfig, error) {
	sort.Strings(imageBundleFiles)

	var (
		// This will hold the merged config from all the image bundles which will be used to import
		// all the images from all the bundles.
		imagesCfg *config.ImagesConfig
		// This will hold the merged config from all the Helm chart bundles which will be used to import
		// all the Helm charts from all the bundles.
		helmChartsCfg *config.HelmChartsConfig
	)

	// Just in case users specify the same bundle twice, keep a track of
	// files that have been extracted already so we only extract each of them once.
	extractedBundles := make(map[string]struct{}, len(imageBundleFiles))

	for _, imageBundleFile := range imageBundleFiles {
		if _, ok := extractedBundles[imageBundleFile]; ok {
			continue
		}
		extractedBundles[imageBundleFile] = struct{}{}

		out.StartOperation(fmt.Sprintf("Unarchiving image bundle %q", imageBundleFile))
		err := archive.UnarchiveToDirectory(imageBundleFile, dest)
		if err != nil {
			out.EndOperationWithStatus(output.Failure())
			return nil, nil, fmt.Errorf(
				"failed to unarchive image bundle: %w",
				err,
			)
		}
		out.EndOperationWithStatus(output.Success())

		imagesCfgFile := filepath.Join(dest, "images.yaml")
		if _, err := os.Lstat(imagesCfgFile); err == nil {
			out.StartOperation("Parsing image bundle config")
			imageBundleCfg, err := config.ParseImagesConfigFile(imagesCfgFile)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return nil, nil, err
			}
			out.V(4).Infof("Images config: %+v", imageBundleCfg)
			out.EndOperationWithStatus(output.Success())

			imagesCfg = imagesCfg.Merge(imageBundleCfg)
		}

		helmChartsCfgFile := filepath.Join(dest, "charts.yaml")
		if _, err := os.Lstat(helmChartsCfgFile); err == nil {
			out.StartOperation("Parsing Helm charts bundle config")
			helmChartsBundleCfg, err := config.ParseHelmChartsConfigFile(helmChartsCfgFile)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return nil, nil, err
			}
			out.V(4).Infof("Helm charts config: %+v", helmChartsBundleCfg)
			out.EndOperationWithStatus(output.Success())

			helmChartsCfg = helmChartsCfg.Merge(helmChartsBundleCfg)
		}
	}

	out.V(4).Infof("Merged images config: %+v", imagesCfg)
	out.V(4).Infof("Merged Helm charts config: %+v", helmChartsCfg)

	return imagesCfg, helmChartsCfg, nil
}
