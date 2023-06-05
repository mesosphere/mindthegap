{ pkgs, ... }:

{
  # https://devenv.sh/basics/
  # env.GREET = "devenv";

  # https://devenv.sh/packages/
  packages = [
    pkgs.bash
    pkgs.coreutils
    pkgs.crane
    pkgs.findutils
    pkgs.ginkgo
    pkgs.git
    pkgs.gnused
    pkgs.gnugrep
    pkgs.gnumake
    pkgs.gojq
    pkgs.golangci-lint
    pkgs.golines
    pkgs.google-cloud-sdk
    pkgs.goreleaser
    pkgs.gotestsum
    pkgs.kubernetes-helm
    pkgs.pre-commit
    pkgs.shfmt
    pkgs.upx
  ];

  # https://devenv.sh/scripts/
  # scripts.hello.exec = "echo hello from $GREET";

  # enterShell = ''
  #   hello
  #   git --version
  # '';

  # https://devenv.sh/languages/
  languages.nix.enable = true;
  languages.go.enable = true;

  # https://devenv.sh/pre-commit-hooks/
  # pre-commit.hooks.shellcheck.enable = true;

  # https://devenv.sh/processes/
  # processes.ping.exec = "ping example.com";

  # See full reference at https://devenv.sh/reference/options/
}
