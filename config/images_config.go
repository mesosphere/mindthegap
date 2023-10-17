// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/distribution/distribution/v3/reference"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
)

// RegistrySyncConfig contains information about a single registry, read from
// the source YAML file.
type RegistrySyncConfig struct {
	// Images map images name to slices with the images' references (tags, digests)
	Images map[string][]string
	// TLS verification mode (enabled by default)
	TLSVerify *bool `yaml:"tlsVerify,omitempty"`
	// Username and password used to authenticate with the registry
	Credentials *types.DockerAuthConfig `yaml:"credentials,omitempty"`
}

func (rsc RegistrySyncConfig) SortedImageNames() []string {
	imageNames := make([]string, 0, len(rsc.Images))
	for imgName := range rsc.Images {
		imageNames = append(imageNames, imgName)
	}
	sort.Strings(imageNames)
	return imageNames
}

func (rsc RegistrySyncConfig) TotalImages() int {
	n := 0
	for _, imgTag := range rsc.Images {
		n += len(imgTag)
	}
	return n
}

func (rsc RegistrySyncConfig) Clone() RegistrySyncConfig {
	images := make(map[string][]string, len(rsc.Images))
	for k, v := range rsc.Images {
		images[k] = append([]string{}, v...)
	}

	var tlsVerify *bool = nil
	if rsc.TLSVerify != nil {
		tlsVerify = ptr.To(*rsc.TLSVerify)
	}

	var creds *types.DockerAuthConfig = nil
	if rsc.Credentials != nil {
		creds = &types.DockerAuthConfig{
			IdentityToken: rsc.Credentials.IdentityToken,
			Username:      rsc.Credentials.Username,
			Password:      rsc.Credentials.Password,
		}
	}

	return RegistrySyncConfig{
		Images:      images,
		TLSVerify:   tlsVerify,
		Credentials: creds,
	}
}

// ImagesConfig contains all registries information read from the source YAML file.
type ImagesConfig map[string]RegistrySyncConfig

func (ic *ImagesConfig) Merge(cfg ImagesConfig) *ImagesConfig {
	if ic == nil && cfg == nil {
		return nil
	}

	if ic == nil {
		return &cfg
	}

	merged := make(ImagesConfig, len(*ic)+len(cfg))

	for k, v := range *ic {
		merged[k] = v.Clone()
	}

	for k, v := range cfg {
		cloned := v.Clone()

		f, ok := merged[k]

		if !ok {
			merged[k] = cloned
			continue
		}

		f.Credentials = cloned.Credentials
		f.TLSVerify = cloned.TLSVerify

		for img, tags := range cloned.Images {
			fImg, ok := f.Images[img]

			if !ok {
				f.Images[img] = tags
				continue
			}

			for _, tag := range tags {
				if !sliceContains(fImg, tag) {
					fImg = append(fImg, tag)
				}
			}

			sort.Strings(fImg)
			f.Images[img] = fImg
		}
	}

	return &merged
}

func sliceContains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}

	return false
}

func (ic ImagesConfig) SortedRegistryNames() []string {
	regNames := make([]string, 0, len(ic))
	for regName := range ic {
		regNames = append(regNames, regName)
	}
	sort.Strings(regNames)
	return regNames
}

func (ic ImagesConfig) TotalImages() int {
	n := 0
	for _, rsc := range ic {
		n += rsc.TotalImages()
	}
	return n
}

func ParseImagesConfigFile(configFile string) (ImagesConfig, error) {
	f, yamlParseErr := os.Open(configFile)
	if yamlParseErr != nil {
		return ImagesConfig{}, fmt.Errorf("failed to read images config file: %w", yamlParseErr)
	}
	defer f.Close()

	var (
		config ImagesConfig
		dec    = yaml.NewDecoder(f)
	)
	dec.KnownFields(true)
	yamlParseErr = dec.Decode(&config)
	if yamlParseErr == nil {
		return config, nil
	}

	config = ImagesConfig{}

	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		return ImagesConfig{}, fmt.Errorf("failed to reset file reader for parsing: %w", seekErr)
	}

	fileScanner := bufio.NewScanner(f)
	fileScanner.Split(bufio.ScanLines)
	for fileScanner.Scan() {
		trimmedLine := strings.TrimSpace(fileScanner.Text())
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		named, nameErr := reference.ParseNormalizedNamed(trimmedLine)
		if nameErr != nil {
			return ImagesConfig{}, fmt.Errorf("failed to parse config file: %w", nameErr)
		}
		namedTagged, ok := named.(reference.NamedTagged)
		if !ok {
			tagged, err := reference.WithTag(named, "latest")
			if err != nil {
				return ImagesConfig{}, fmt.Errorf("invalid image name %q: %w", named, err)
			}
			namedTagged = tagged
		}

		registry := reference.Domain(namedTagged)
		name := reference.Path(named)
		tag := namedTagged.Tag()

		if _, found := config[registry]; !found {
			config[registry] = RegistrySyncConfig{Images: map[string][]string{}}
		}
		config[registry].Images[name] = append(config[registry].Images[name], tag)
	}

	return config, nil
}

func WriteSanitizedImagesConfig(cfg ImagesConfig, fileName string) error {
	for regName, regConfig := range cfg {
		regConfig.Credentials = nil
		regConfig.TLSVerify = nil
		cfg[regName] = regConfig
	}

	return writeYAMLToFile(cfg, fileName)
}

func writeYAMLToFile(obj interface{}, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file to write config to: %w", err)
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	defer enc.Close()
	enc.SetIndent(2)
	if err := enc.Encode(obj); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}
