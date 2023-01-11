<!--
 Copyright 2021 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

<h1 align="center"><img src="mindthegap.svg" alt="mindthegap" width="300"/></h1>

![GitHub](https://img.shields.io/github/license/mesosphere/mindthegap?style=flat-square)

`mindthegap` provides utilities to manage air-gapped image bundles, both
creating image bundles and seeding images from a bundle into an existing
OCI registry.

## Usage

### Image bundles

#### Creating an image bundle

```shell
mindthegap create image-bundle --images-file <path/to/images.yaml> \
  --platform <platform> [--platform <platform> ...] \
  [--output-file <path/to/output.tar>]
```

See the [example images.yaml](images-example.yaml) for the structure of the
images config file. You can also provide the images file in a simple file with
an image per line, e.g.

```plain
nginx:1.21.5
test.registry2.io/test-image6:atag
```

Note that images from Docker Hub must be prefixed with `docker.io` and those "official" images
must have the `library` namespace specified.

Platform can be specified multiple times. Supported platforms:

```plain
linux/amd64
linux/arm64
windows/amd64
windows/arm64
```

All images in the images config file must support all the requested platforms.

The output file will be a tarball that can be seeded into a registry,
or that can be untarred and used as the storage directory for a Docker registry
served via `registry:2`.

#### Pushing an image bundle

```shell
mindthegap push image-bundle --image-bundle <path/to/images.tar> \
  --to-registry <registry.address> \
  [--to-registry-insecure-skip-tls-verify]
```

All images in the image bundle tar file will be pushed to the target docker registry.

#### Serving an image bundle

```shell
mindthegap serve image-bundle --image-bundle <path/to/images.tar> \
  [--listen-address <listen.address>] \
  [--listen-port <listen.port>]
```

Start a Docker registry serving the contents of the image bundle. Note that the Docker registry will
be in read-only mode to reflect the source of the data being a static tarball so pushes to this
registry will fail.

#### Importing an image bundle into containerd

```shell
mindthegap import image-bundle --image-bundle <path/to/images.tar> \
  [--containerd-namespace <containerd.namespace]
```

Import the images from the image bundle into containerd in the specified namespace. If
`--containerd-namespace` is not specified, images will be imported into `k8s.io` namespace. This
command requires `ctr` to be in the `PATH`.

### Helm chart bundles

#### Creating a Helm chart bundle

```shell
mindthegap create helm-bundle --helm-charts-file <path/to/helm-charts.yaml> \
  [--output-file <path/to/output.tar>]
```

See the [example helm-charts.yaml](helm-example.yaml) for the structure of the
Helm charts config file.

The output file will be a tarball that can be seeded into a registry,
or that can be untarred and used as the storage directory for a Docker registry
served via `registry:2`.

#### Serving a Helm chart bundle

```shell
mindthegap serve helm-bundle --helm-bundle <path/to/helm-charts.tar> \
  [--listen-address <addr>] \
  [--list-port <port>] \
  [--tls-cert-file <path/to/cert/file> --tls-private-key-file <path/to/key/file>]
```

Start a Docker registry serving the contents of the image bundle. Note that the Docker registry will
be in read-only mode to reflect the source of the data being a static tarball so pushes to this
registry will fail.

## How does it work?

`mindthegap` starts up a [Docker registry](https://docs.docker.com/registry/)
and then uses [`crane`](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md)
as a library to copy the specified images for all specified platforms into the running registry. The
resulting registry storage is then tarred up, resulting in a tarball of the specified images.

The resulting tarball can be loaded into a running Docker registry, or
be used as the initial storage for running your own registry from via Docker
or in a Kubernetes cluster.

## Building

### Building the CLI

Build the CLI using `make build-snapshot` that will output binary into
`dist/mindthegap_$(GOOS)_$(GOARCH)/mindthegap` and put it in `$PATH`.
