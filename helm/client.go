// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-getter"
	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

type Client struct {
	tempDir string
	out     output.Output
}

type CleanupFunc func() error

func NewClient(out output.Output) (*Client, CleanupFunc) {
	tempDir, err := os.MkdirTemp("", ".helm-bundle-*")
	if err != nil {
		panic(err)
	}

	c := &Client{
		out:     out,
		tempDir: tempDir,
	}
	return c, func() error {
		return os.RemoveAll(filepath.Dir(c.tempDir))
	}
}

func (c *Client) doNotUntarOpt() action.PullOpt {
	return func(p *action.Pull) {
		p.Untar = false
	}
}

func (c *Client) destDir(outputDir string) action.PullOpt {
	return func(p *action.Pull) {
		p.DestDir = outputDir
	}
}

func (c *Client) tempRepositoryCache() action.PullOpt {
	return func(p *action.Pull) {
		if p.Settings == nil {
			p.Settings = &cli.EnvSettings{}
		}
		p.Settings.RepositoryCache = c.tempDir
	}
}

func (c *Client) repoURL(repoURL string) action.PullOpt {
	return func(p *action.Pull) {
		p.RepoURL = repoURL
	}
}

func (c *Client) GetChartFromRepo(
	outputDir, repoURL, chartName string,
	chartVersions ...string,
) error {
	pull := action.NewPullWithOpts(
		action.WithConfig(&action.Configuration{Log: c.out.V(4).Infof}),
		c.doNotUntarOpt(),
		c.destDir(outputDir),
		c.tempRepositoryCache(),
		c.repoURL(repoURL),
	)
	for _, v := range chartVersions {
		pull.Version = v
		helmOutput, err := pull.Run(chartName)
		if err != nil {
			return fmt.Errorf(
				"failed to fetch chart %s:%s from %s: %w, output:\n\n%s",
				chartName,
				chartVersions,
				repoURL,
				err,
				helmOutput,
			)
		}
		if helmOutput != "" {
			c.out.V(4).Info(helmOutput)
		}
	}

	return nil
}

func (c *Client) GetChartFromURL(outputDir, chartURL, workingDir string) error {
	getters := make(map[string]getter.Getter, len(getter.Getters))
	for scheme, getter := range getter.Getters {
		getters[scheme] = getter
	}
	copyFileGetter := new(getter.FileGetter)
	copyFileGetter.Copy = true
	getters["file"] = copyFileGetter

	u, err := url.Parse(chartURL)
	if err != nil {
		return fmt.Errorf("invalid chart URL: %w", err)
	}
	q := u.Query()
	q.Set("archive", "false")
	u.RawQuery = q.Encode()

	dst := filepath.Join(outputDir, filepath.Base(chartURL))
	err = getter.GetFile(dst, u.String(), func(c *getter.Client) error {
		c.Pwd = workingDir
		return nil
	}, func(c *getter.Client) error {
		c.Getters = getters
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to fetch chart from %s: %w", chartURL, err)
	}
	return nil
}

func (c *Client) CreateHelmRepoIndex(dir string) error {
	indexFile, err := repo.IndexDirectory(dir, "")
	if err != nil {
		return fmt.Errorf("failed to create Helm repo index file: %w", err)
	}
	if err := indexFile.WriteFile(filepath.Join(dir, "index.yaml"), 0o644); err != nil {
		return fmt.Errorf("failed to write Helm repo index file: %w", err)
	}
	return nil
}
