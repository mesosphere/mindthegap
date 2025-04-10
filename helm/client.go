// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/distribution/reference"
	"github.com/hashicorp/go-getter"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	helmgetter "helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/klog/v2"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
)

const OCIScheme = registry.OCIScheme

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
		return os.RemoveAll(c.tempDir)
	}
}

func DoNotUntarOpt() action.PullOpt {
	return func(p *action.Pull) {
		p.Untar = false
	}
}

func DestDirOpt(outputDir string) action.PullOpt {
	return func(p *action.Pull) {
		p.DestDir = outputDir
	}
}

func TempRepositoryCacheOpt(tempDir string) action.PullOpt {
	return func(p *action.Pull) {
		if p.Settings == nil {
			p.Settings = &cli.EnvSettings{}
		}
		p.Settings.RepositoryCache = tempDir
	}
}

func RepoURLOpt(repoURL string) action.PullOpt {
	return func(p *action.Pull) {
		p.RepoURL = repoURL
	}
}

func ChartVersionOpt(chartVersion string) action.PullOpt {
	return func(p *action.Pull) {
		p.Version = chartVersion
	}
}

func UsernamePasswordOpt(username, password string) action.PullOpt {
	return func(p *action.Pull) {
		p.Username = username
		p.Password = password
	}
}

func InsecureSkipTLSverifyOpt() action.PullOpt {
	return func(p *action.Pull) {
		p.InsecureSkipTLSverify = true
	}
}

func CAFileOpt(caFile string) action.PullOpt {
	return func(p *action.Pull) {
		p.CaFile = caFile
	}
}

func (c *Client) GetChartFromRepo(
	outputDir, repoURL, chartName, chartVersion string,
	extraPullOpts ...action.PullOpt,
) (string, error) {
	cfg := &action.Configuration{Log: c.out.V(4).Infof}

	pull := action.NewPullWithOpts(
		append(
			extraPullOpts,
			action.WithConfig(cfg),
			DoNotUntarOpt(),
			DestDirOpt(outputDir),
			TempRepositoryCacheOpt(c.tempDir),
			RepoURLOpt(repoURL),
			ChartVersionOpt(chartVersion),
		)...,
	)

	// Charts pulled from OCI registries will have the scheme "oci://" for the chart name.
	// We can use the built-in downloader to fetch these charts.
	if strings.HasPrefix(chartName, OCIScheme) {
		helmOutput, err := pull.Run(chartName)
		if err != nil {
			return "", fmt.Errorf(
				"failed to fetch chart %s:%s from %s: %w, output:\n\n%s",
				chartName,
				chartVersion,
				repoURL,
				err,
				helmOutput,
			)
		}
		if helmOutput != "" {
			c.out.V(4).Info(helmOutput)
		}

		return filepath.Join(
			outputDir,
			fmt.Sprintf("%s-%s.tgz", filepath.Base(chartName), chartVersion),
		), nil
	}

	// For non-OCI charts, we need to discover the chart URL first to be able to handle
	// different chart names to the expected `<chartName>-<chartVersion>.tgz` format.
	chartURL, err := repo.FindChartInAuthAndTLSAndPassRepoURL(
		pull.RepoURL,
		pull.Username,
		pull.Password,
		chartName,
		chartVersion,
		pull.CertFile,
		pull.KeyFile,
		pull.CaFile,
		pull.InsecureSkipTLSverify,
		pull.PassCredentialsAll,
		helmgetter.All(pull.Settings),
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to discover chart URL for %s:%s from %s: %w",
			chartName,
			chartVersion,
			repoURL,
			err,
		)
	}

	return c.GetChartFromURL(outputDir, chartURL, c.tempDir)
}

func (c *Client) GetChartFromURL(outputDir, chartURL, workingDir string) (string, error) {
	// Charts pulled from OCI registries will have the scheme "oci://" for the chart name.
	// We can use the built-in Helm downloader to fetch these charts.
	if strings.HasPrefix(chartURL, OCIScheme) {
		return c.getChartFromOCIURL(outputDir, chartURL)
	}

	getters := make(map[string]getter.Getter, len(getter.Getters))
	for scheme, getter := range getter.Getters {
		getters[scheme] = getter
	}
	copyFileGetter := new(getter.FileGetter)
	copyFileGetter.Copy = true
	getters["file"] = copyFileGetter

	u, err := url.Parse(chartURL)
	if err != nil {
		return "", fmt.Errorf("invalid chart URL: %w", err)
	}
	q := u.Query()
	q.Set("archive", "false")
	u.RawQuery = q.Encode()

	dst := filepath.Join(outputDir, filepath.Base(chartURL))
	err = getter.GetFile(dst, u.String(), func(getterClient *getter.Client) error {
		getterClient.Pwd = workingDir
		return nil
	}, func(getterClient *getter.Client) error {
		getterClient.Getters = getters
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch chart from %s: %w", chartURL, err)
	}
	return filepath.Join(outputDir, filepath.Base(chartURL)), nil
}

func (c *Client) getChartFromOCIURL(outputDir, chartURL string) (string, error) {
	ociURL, err := url.Parse(chartURL)
	if err != nil {
		return "", fmt.Errorf("invalid OCI chart URL %q: %w", chartURL, err)
	}

	ociRef, err := reference.ParseNormalizedNamed(strings.TrimPrefix(chartURL, ociURL.Scheme+"://"))
	if err != nil {
		return "", fmt.Errorf("invalid OCI chart URL %q: %w", chartURL, err)
	}

	taggedOCIRef, ok := ociRef.(reference.NamedTagged)
	if !ok {
		tagged, err := reference.WithTag(ociRef, "latest")
		if err != nil {
			return "", fmt.Errorf("invalid OCI chart URL %q: %w", chartURL, err)
		}
		taggedOCIRef = tagged
	}

	return c.GetChartFromRepo(
		outputDir,
		"",
		OCIScheme+"://"+taggedOCIRef.Name(),
		taggedOCIRef.Tag(),
	)
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

func (c *Client) PushHelmChartToOCIRegistry(src, ociDest string) error {
	registryClient, err := registry.NewClient(registry.ClientOptDebug(klog.V(4).Enabled()))
	if err != nil {
		return fmt.Errorf("failed to create registry client for Helm chart push: %w", err)
	}

	push := action.NewPushWithOpts(action.WithPushConfig(&action.Configuration{
		RegistryClient: registryClient,
	}))

	helmOutput, err := push.Run(src, ociDest)
	if err != nil {
		return fmt.Errorf(
			"failed to push chart %s to %s: %w, output:\n\n%s",
			src,
			ociDest,
			err,
			helmOutput,
		)
	}

	if helmOutput != "" {
		c.out.V(4).Info(helmOutput)
	}

	return nil
}

func LoadChart(chartPath string) (*chart.Chart, error) {
	chrt, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}
	return chrt, nil
}
