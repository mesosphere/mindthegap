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

### Creating a bundle

```shell
mindthegap create bundle \
  [--images-file <path/to/images.yaml>] \
  [--platform <platform> [--platform <platform> ...]] \
  [--helm-charts-file <path/to/helm-charts.yaml>] \
  [--oci-artifacts-file <path/to/helm-charts.yaml>] \
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
or that can be untarred and used as the storage directory for an OCI registry
served via `registry:2`.

See the [example helm-charts.yaml](helm-example.yaml) for the structure of the
Helm charts config file.  You can also provide the images file in a simple file with
a chart URL per line, e.g.

```plain
oci://ghcr.io/stefanprodan/charts/podinfo:6.1.0
```

It is also possible to include OCI artifacts that are not OCI images.
This is useful for bundling Flux kustomizations, Helm Charts directly from OCI
registries, and any arbitrary OCI artifacts. To include an OCI artifacts, specify
the `--oci-artifacts-file` path. The format of the provided file matches the
`--images-file` format. The `--platform` flag has no effect on OCI artifacts.

The OCI artifacts with image index are not supported.

### Pushing a bundle

```shell
mindthegap push bundle --bundle <path/to/bundle.tar> \
  --to-registry <registry.address> \
  [--to-registry-insecure-skip-tls-verify]
```

All images in an image bundle tar file, or Helm charts in a chart bundle, will be pushed to the target OCI registry.

#### Existing tag behaviour

When pushing to a registry which could already contain tags that are included in the bundle, the behaviour can be
specified via the `--on-existing-tag` flag. The following strategies are available:

- `overwrite`: Overwrite the tag with the contents from the bundle (Default)
- `error`: Return an error if a matching tag already exists
- `skip`: Do not push the tag if it already exists
- `merge-with-retain`: Merge the image index from the bundle with the existing tag, retaining any platforms that already
  exist in the registry
- `merge-with-overwrite`: Merge the image index from the bundle with the existing tag, overwriting any platforms that
  already exist in the registry

### Serving a bundle

```shell
mindthegap serve bundle --bundle <path/to/bundle.tar> \
  [--listen-address <listen.address>] \
  [--listen-port <listen.port>]
```

Start an OCI registry serving the contents of the image bundle or Helm charts bundle. Note that the OCI registry will
be in read-only mode to reflect the source of the data being a static tarball so pushes to this
registry will fail.

### Importing an image bundle into containerd

```shell
mindthegap import image-bundle --image-bundle <path/to/images.tar> \
  [--containerd-namespace <containerd.namespace]
```

Import the images from the image bundle into containerd in the specified namespace. If
`--containerd-namespace` is not specified, images will be imported into `k8s.io` namespace. This
command requires `ctr` to be in the `PATH`.

## How does it work?

`mindthegap` starts up an [OCI registry](https://docs.docker.com/registry/)
and then uses [`crane`](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md)
as a library to copy the specified images for all specified platforms into the running registry. The
resulting registry storage is then tarred up, resulting in a tarball of the specified images.

The resulting tarball can be loaded into a running OCI registry, or
be used as the initial storage for running your own registry via Docker
or in a Kubernetes cluster.

## Contributing

This project uses <https://www.jetpack.io/devbox/> to create a reproducible build environment. If you do not have
`devbox` configured, then the following instructions should work for you. For further details, see
<https://www.jetpack.io/devbox/docs/installing_devbox/>.

### Integrate with `direnv` for automatic shell integration

Install direnv: <https://direnv.net/docs/installation.html#from-system-packages>.

Hook direnv into your shell if you haven't already: <https://direnv.net/docs/hook.html>.

## Building the CLI

`mindthegap` uses [`task`](https://taskfile.dev/) for running build tasks. `task` will be automatically available when
the devbox environment is correctly set up.

Build the CLI using `task build:snapshot` that will output binary into
`./dist/mindthegap_$(GOOS)_$(GOARCH)/mindthegap`.
