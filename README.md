# Go Repository Template

This is a GitHub repository template for Go. It has been created for ease-of-use for anyone who wants to build binaries, Docker images, and publish Github releases.

It includes:

- build automation via [Make](https://www.gnu.org/software/make),
- dependency management using [Go Modules](https://github.com/golang/go/wiki/Modules),
- linting with [golangci-lint](https://github.com/golangci/golangci-lint),
- unit testing with [testify](https://github.com/stretchr/testify), [race detector](https://blog.golang.org/race-detector), code covarage [HTML report](https://blog.golang.org/cover) using [go-acc](https://github.com/ory/go-acc) for accurate reporting,
- releasing using [GoReleaser](https://github.com/goreleaser/goreleaser),
- dependencies scanning and updating thanks to [Dependabot](https://dependabot.com),
- [Visual Studio Code](https://code.visualstudio.com) configuration with [Go](https://code.visualstudio.com/docs/languages/go) and [Remote Container](https://code.visualstudio.com/docs/remote/containers) support.

This work is based on the upstream [Go Repository Template](https://github.com/golang-templates/seed).

## Usage

1. Click the `Use this template` button (alt. clone or download this repository).
1. Replace all occurences of `mesosphere/golang-repository-template` to `your_org/repo_name` in all files.
1. Rename folder `cmd/seed` to `cmd/app_name` and update [.goreleaser.yml](.goreleaser.yml) accordingly.
1. Update [LICENSE](LICENSE) and [README.md](README.md).

## Build and Test

Tip: to see all available make targets with descriptions, simply run `make`.

To run unit tests, run `make test`.

To run integration tests, run `make integration-test`.

To build development binaries (specified in [`.goreleaser.yml`](.goreleaser.yml)), run `make build-snapshot` This will output binaries in [`dist`](dist) for all configured platforms.

To build a release snapshot locally (including all configured packages, etc in [`.goreleaser.yml`](.goreleaser.yml)), run `make release-snapshot`.

To build a full release, including publishing release artifacts, run `make release`.

To run any command inside the Docker container used in CI, run `make docker run="make <target>"`.

To run `seed` command without building output binaries, run `go run ./cmd/seed/main.go <subcommands_and_flags>`

## Release

_CAUTION_: Make sure to understand the consequences before you bump the major version. More info: [Go Wiki](https://github.com/golang/go/wiki/Modules#releasing-modules-v2-or-higher), [Go Blog](https://blog.golang.org/v2-go-modules).

## Maintainance

Remember to update Go version in [devcontainer.json](.devcontainer/devcontainer.json).

Notable files:
- [devcontainer.json](.devcontainer/devcontainer.json) - Visual Studio Code Remote Container configuration,
- [.github/dependabot.yml](.github/dependabot.yml) - Dependabot configuration,
- [.vscode](.vscode) - Visual Studio Code configuration files,
- [.golangci.yml](.golangci.yml) - golangci-lint configuration,
- [.goreleaser.yml](.goreleaser.yml) - GoReleaser configuration,
- [Makefile](Makefile) - Make targets used for development, and [.vscode/tasks.json](.vscode/tasks.json),
- [go.mod](go.mod) - [Go module definition](https://github.com/golang/go/wiki/Modules#gomod),
- [tools.go](tools.go) - [build tools](https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module).
- [`cmd`](cmd) - Commands and their respective subcommands, thin wrappers over the library code in [`pkg`](pkg).
- [`pkg`](pkg) - All library code for this project.

## FAQ

### Why Visual Studio Code editor configuration

Developers that use Visual Studio Code can take advantage of the editor configuration. While others do not have to care about it. Setting configs for each repo is unnecessary time consuming. VS Code is the most popular Go editor ([survey](https://blog.golang.org/survey2019-results)) and it is officially [supported by the Go team](https://blog.golang.org/vscode-go).

You can always remove the [.devcontainer](.devcontainer) and [.vscode](.vscode) directories if it really does not help you.

### How can I create a Docker image, deb/rpm/snap package, Homebrew Tap, Scoop App Manifest etc.

Take a look at GoReleaser [docs](https://goreleaser.com/customization/) as well as [its repo](https://github.com/goreleaser/goreleaser/) how it is dogfooding its functionality.

### How can I create a library instead of an application

You can change the [.goreleaser.yml](.goreleaser.yml) to contain:

```yaml
build:
  skip: true
release:
  github:
  prerelease: auto
```

Alternatively, you can completly remove the usage of GoReleaser if you prefer handcrafted release notes.

## Contributing

Simply create an issue or a pull request.
