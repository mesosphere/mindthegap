// Copyright 2021 Mesosphere, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"

	"github.com/containers/image/types"
	"gopkg.in/yaml.v3"
)

// RegistrySyncConfig contains information about a single registry, read from
// the source YAML file.
type RegistrySyncConfig struct {
	// Images map images name to slices with the images' references (tags, digests)
	Images map[string][]string
	// TLS verification mode (enabled by default)
	TLSVerify *bool `yaml:"tls-verify,omitempty"`
	// Username and password used to authenticate with the registry
	Credentials *types.DockerAuthConfig `yaml:"credentials,omitempty"`
}

// SourceConfig contains all registries information read from the source YAML file.
type SourceConfig map[string]RegistrySyncConfig

func ParseFile(configFile string) (SourceConfig, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return SourceConfig{}, fmt.Errorf("failed to read images config file: %w", err)
	}
	var (
		config SourceConfig
		dec    = yaml.NewDecoder(f)
	)
	dec.KnownFields(true)
	err = dec.Decode(&config)
	_ = f.Close()
	if err != nil {
		return SourceConfig{}, fmt.Errorf("failed to parse config file: %w", err)
	}
	return config, nil
}

func WriteSanitizedConfig(cfg SourceConfig, fileName string) error {
	for regName, regConfig := range cfg {
		regConfig.Credentials = nil
		cfg[regName] = regConfig
	}

	f, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file to write sanitized config to: %w", err)
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	defer enc.Close()
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("failed to write sanitized config: %w", err)
	}
	return nil
}
