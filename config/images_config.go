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
	for regName := range rsc.Images {
		imageNames = append(imageNames, regName)
	}
	sort.Strings(imageNames)
	return imageNames
}

// ImagesConfig contains all registries information read from the source YAML file.
type ImagesConfig map[string]RegistrySyncConfig

func (ic ImagesConfig) SortedRegistryNames() []string {
	regNames := make([]string, 0, len(ic))
	for regName := range ic {
		regNames = append(regNames, regName)
	}
	sort.Strings(regNames)
	return regNames
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
		named, nameErr := reference.ParseNamed(trimmedLine)
		if nameErr != nil {
			return ImagesConfig{}, fmt.Errorf("failed to parse config file: %w", yamlParseErr)
		}
		namedTagged, ok := named.(reference.NamedTagged)
		if !ok {
			return ImagesConfig{}, fmt.Errorf("failed to parse config file: %w", yamlParseErr)
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
