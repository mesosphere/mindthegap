// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package images

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func ManifestListForImage(
	img string,
	platforms []string,
	opts ...remote.Option,
) (v1.ImageIndex, error) {
	ref, err := name.ParseReference(img)
	if err != nil {
		return nil, fmt.Errorf("invalid image reference %q: %w", img, err)
	}
	desc, err := remote.Get(ref, opts...)
	if err != nil {
		localImage, localErr := daemon.Image(ref)
		if localErr != nil {
			return nil, fmt.Errorf(
				"failed to read image descriptor for %q from registry: %w",
				img,
				err,
			)
		}

		return indexForSinglePlatformImage(ref, localImage, platforms...)
	}

	switch {
	case desc.MediaType.IsIndex():
		index, err := desc.ImageIndex()
		if err != nil {
			return nil, fmt.Errorf("failed to read image index for %q: %w", img, err)
		}
		return retainOnlyRequestedPlatformsInIndex(index, platforms...)
	case desc.MediaType.IsImage():
		image, err := desc.Image()
		if err != nil {
			return nil, fmt.Errorf("failed to read image for %q: %w", img, err)
		}
		return indexForSinglePlatformImage(ref, image, platforms...)
	default:
		return nil, fmt.Errorf(
			"unexpected media type in descriptor for image %q: %v",
			img,
			desc.MediaType,
		)
	}
}

func retainOnlyRequestedPlatformsInIndex(
	index v1.ImageIndex,
	platforms ...string,
) (v1.ImageIndex, error) {
	v1Platforms := make([]v1.Platform, 0, len(platforms))
	for _, p := range platforms {
		v1P, err := v1.ParsePlatform(p)
		if err != nil {
			return nil, fmt.Errorf("invalid platform %q: %w", p, err)
		}
		v1Platforms = append(v1Platforms, *v1P)
	}

	if len(platforms) == 0 {
		return index, nil
	}

	return mutate.RemoveManifests(
		index,
		notMatcher(platformsIgnoringVariantIfNotSpecified(v1Platforms...)),
	), nil
}

func notMatcher(matcher match.Matcher) match.Matcher {
	return func(desc v1.Descriptor) bool {
		return !matcher(desc)
	}
}

func platformsIgnoringVariantIfNotSpecified(platforms ...v1.Platform) match.Matcher {
	return func(desc v1.Descriptor) bool {
		if desc.Platform == nil {
			return false
		}
		for _, platform := range platforms {
			if desc.Platform.Equals(platform) {
				return true
			}
			if platform.Variant == "" &&
				platform.OS == desc.Platform.OS &&
				platform.Architecture == desc.Platform.Architecture {
				return true
			}
		}
		return false
	}
}

func indexForSinglePlatformImage(
	ref name.Reference,
	img v1.Image,
	platforms ...string,
) (v1.ImageIndex, error) {
	if len(platforms) > 1 {
		return nil,
			fmt.Errorf(
				"reference %q is a single platform image, cannot create an index with multiple platforms (%v) as requested",
				ref,
				platforms,
			)
	}

	imgConfig, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get image config for image %q: %w", ref, err)
	}

	imgPlatform := v1.Platform{
		OS:           imgConfig.OS,
		OSVersion:    imgConfig.OSVersion,
		Architecture: imgConfig.Architecture,
		Variant:      imgConfig.Variant,
	}

	var index v1.ImageIndex = empty.Index
	index = mutate.AppendManifests(
		index,
		mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: &imgPlatform,
			},
		},
	)

	imgMediaType, err := img.MediaType()
	if err != nil {
		return nil, fmt.Errorf("failed to get image media type for image %q: %w", ref, err)
	}
	idxMediaType := types.OCIImageIndex
	if strings.Contains(string(imgMediaType), types.DockerVendorPrefix) {
		idxMediaType = types.DockerManifestList
	}

	index = mutate.IndexMediaType(index, idxMediaType)

	if len(platforms) == 0 {
		return index, nil
	}

	v1Platform, err := v1.ParsePlatform(platforms[0])
	if err != nil {
		return nil, fmt.Errorf("invalid platform %q: %w", platforms[0], err)
	}

	imgPlatformForComparison := imgPlatform
	if v1Platform.Variant == "" {
		imgPlatformForComparison.Variant = ""
	}

	if !imgPlatformForComparison.Equals(*v1Platform) {
		return nil, fmt.Errorf(
			"requested image %q does not match requested platform %q (image is for %q)",
			ref,
			v1Platform,
			imgPlatform,
		)
	}

	return index, nil
}
