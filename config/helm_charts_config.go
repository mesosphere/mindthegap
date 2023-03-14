// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
)

// HelmRepositorySyncConfig contains information about a single repository, read from
// the source YAML file.
type HelmRepositorySyncConfig struct {
	// RepoURL is the URL for the repository.
	RepoURL string `yaml:"repoURL,omitempty"`
	// Username holds the username for the repository.
	Username string `yaml:"username,omitempty"`
	// Password holds the password for the repository.
	Password string `yaml:"password,omitempty"`
	// TLS verification mode (enabled by default)
	TLSVerify *bool `yaml:"tlsVerify,omitempty"`
	// Charts map charts name to slices with the chart versions.
	Charts map[string][]string `yaml:"charts,omitempty"`
}

func (c HelmRepositorySyncConfig) Clone() HelmRepositorySyncConfig {
	charts := make(map[string][]string, len(c.Charts))
	for k, v := range c.Charts {
		charts[k] = append([]string{}, v...)
	}

	var tlsVerify *bool = nil
	if c.TLSVerify != nil {
		tlsVerify = pointer.Bool(*c.TLSVerify)
	}

	return HelmRepositorySyncConfig{
		Charts:    charts,
		TLSVerify: tlsVerify,
		Username:  c.Username,
		Password:  c.Password,
	}
}

func (c HelmRepositorySyncConfig) SortedChartNames() []string {
	chartNames := make([]string, 0, len(c.Charts))
	for chartName := range c.Charts {
		chartNames = append(chartNames, chartName)
	}
	sort.Strings(chartNames)
	return chartNames
}

// HelmChartsConfig contains all helm charts information read from the source YAML file.
type HelmChartsConfig struct {
	Repositories map[string]HelmRepositorySyncConfig `yaml:"repositories,omitempty"`
	ChartURLs    []string                            `yaml:"chartURLs,omitempty"`
}

func (c HelmChartsConfig) SortedRepositoryNames() []string {
	repoNames := make([]string, 0, len(c.Repositories))
	for repoName := range c.Repositories {
		repoNames = append(repoNames, repoName)
	}
	sort.Strings(repoNames)
	return repoNames
}

func (c *HelmChartsConfig) Merge(cfg HelmChartsConfig) *HelmChartsConfig {
	if c == nil {
		return &cfg
	}

	var mergedRepos map[string]HelmRepositorySyncConfig

	if c.Repositories != nil || cfg.Repositories != nil {
		mergedRepos = make(
			map[string]HelmRepositorySyncConfig,
			len(c.Repositories)+len(cfg.Repositories),
		)

		for k, v := range c.Repositories {
			mergedRepos[k] = v.Clone()
		}

		for k, v := range cfg.Repositories {
			cloned := v.Clone()

			f, ok := mergedRepos[k]

			if !ok {
				mergedRepos[k] = cloned
				continue
			}

			f.Username = cloned.Username
			f.Password = cloned.Password
			f.TLSVerify = cloned.TLSVerify

			for chrt, versions := range cloned.Charts {
				fVersion, ok := f.Charts[chrt]

				if !ok {
					f.Charts[chrt] = versions
					continue
				}

				for _, version := range versions {
					if !sliceContains(fVersion, version) {
						fVersion = append(fVersion, version)
					}
				}

				sort.Strings(fVersion)
				f.Charts[chrt] = fVersion
			}
		}
	}

	var mergedChartURLs []string
	if c.ChartURLs != nil || cfg.ChartURLs != nil {
		mergedChartURLs = sets.NewString(append(c.ChartURLs, cfg.ChartURLs...)...).List()
	}

	return &HelmChartsConfig{
		Repositories: mergedRepos,
		ChartURLs:    mergedChartURLs,
	}
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
	if yamlParseErr != nil {
		return HelmChartsConfig{}, fmt.Errorf("failed to parse config file: %w", yamlParseErr)
	}

	return config, nil
}

func WriteSanitizedHelmChartsConfig(cfg HelmChartsConfig, fileName string) error {
	for regName, regConfig := range cfg.Repositories {
		regConfig.Username = ""
		regConfig.Password = ""
		regConfig.TLSVerify = nil
		cfg.Repositories[regName] = regConfig
	}

	cfg.ChartURLs = nil

	return writeYAMLToFile(cfg, fileName)
}
