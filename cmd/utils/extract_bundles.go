// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
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
) (config.ImagesConfig, error) {
	sort.Strings(imageBundleFiles)

	// This will hold the merged config from all the image bundles which will be used to import
	// all the images from all the bundles.
	var cfg config.ImagesConfig

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
			out.EndOperation(false)
			return config.ImagesConfig{}, fmt.Errorf("failed to unarchive image bundle: %w", err)
		}
		out.EndOperation(true)

		out.StartOperation("Parsing image bundle config")
		bundleCfg, err := config.ParseImagesConfigFile(
			filepath.Join(dest, "images.yaml"),
		)
		if err != nil {
			out.EndOperation(false)
			return config.ImagesConfig{}, err
		}
		out.V(4).Infof("Images config: %+v", bundleCfg)
		out.EndOperation(true)

		cfg = cfg.Merge(bundleCfg)
	}

	out.V(4).Infof("Merged images config: %+v", cfg)

	return cfg, nil
}
