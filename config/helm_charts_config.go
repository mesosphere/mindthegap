// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// HelmRepositorySyncConfig contains information about a single repository, read from
// the source YAML file.
type HelmRepositorySyncConfig struct {
	// Charts map charts name to slices with the chart versions
	Charts map[string][]string
}

// HelmChartsConfig contains all helm charts information read from the source YAML file.
type HelmChartsConfig struct {
	Repositories map[string]HelmRepositorySyncConfig `yaml:"repositories,omitempty"`
	ChartURLs    []string                            `yaml:"chartURLs,omitempty"`
}

func ParseHelmChartsConfigFile(configFile string) (HelmChartsConfig, error) {
	f, yamlParseErr := os.Open(configFile)
	if yamlParseErr != nil {
		return HelmChartsConfig{}, fmt.Errorf(
			"failed to read helm charts config file: %w",
			yamlParseErr,
		)
	}
	defer f.Close()

	var (
		config HelmChartsConfig
		dec    = yaml.NewDecoder(f)
	)
	dec.KnownFields(true)
	yamlParseErr = dec.Decode(&config)
	if yamlParseErr == nil {
		return HelmChartsConfig{}, fmt.Errorf("failed to parse config file: %w", yamlParseErr)
	}

	return config, nil
}

func WriteHelmChartsConfig(cfg HelmChartsConfig, fileName string) error {
	return writeYAMLToFile(cfg, fileName)
}
