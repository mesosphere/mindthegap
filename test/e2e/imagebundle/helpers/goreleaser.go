// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Artifact is a goreleaser type defines a single artifact.
// Copied from https://github.com/goreleaser/goreleaser/blob/v1.13.1/internal/artifact/artifact.go#L159-L170
type Artifact struct {
	Name    string         `json:"name,omitempty"`
	Path    string         `json:"path,omitempty"`
	Goos    string         `json:"goos,omitempty"`
	Goarch  string         `json:"goarch,omitempty"`
	Goarm   string         `json:"goarm,omitempty"`
	Gomips  string         `json:"gomips,omitempty"`
	Goamd64 string         `json:"goamd64,omitempty"`
	Type    string         `json:"type,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

type Artifacts []Artifact

func (as Artifacts) SelectBinary(name, goos, goarch string) (Artifact, bool) {
	for i := range as {
		a := as[i]
		if a.Type == "Binary" && a.Name == name && a.Goos == goos && a.Goarch == goarch {
			return a, true
		}
	}

	return Artifact{}, false
}

func (as Artifacts) SelectDockerImage(namePrefix, goos, goarch string) (Artifact, bool) {
	for i := range as {
		a := as[i]
		if a.Type == "Docker Image" && strings.HasPrefix(a.Name, namePrefix) && a.Goos == goos &&
			a.Goarch == goarch {
			return a, true
		}
	}

	return Artifact{}, false
}

func ParseArtifactsFile(file string) (Artifacts, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ParseArtifacts(f)
}

func ParseArtifacts(r io.Reader) (Artifacts, error) {
	var as Artifacts
	if err := json.NewDecoder(r).Decode(&as); err != nil {
		return nil, err
	}
	return as, nil
}
