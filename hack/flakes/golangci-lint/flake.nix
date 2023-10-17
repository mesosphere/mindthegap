# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

{
  description = "Fast linters Runner for Go";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils/v1.0.0";
  };

  outputs =
    { self
    , nixpkgs
    , utils
    ,
    }:
    let
      appReleaseVersion = "1.54.2";
      appReleaseBinaries = {
        "x86_64-linux" = {
          fileName = "golangci-lint-${appReleaseVersion}-linux-amd64.tar.gz";
          sha256 = "17c9ca05253efe833d47f38caf670aad2202b5e6515879a99873fabd4c7452b3";
        };
        "x86_64-darwin" = {
          fileName = "golangci-lint-${appReleaseVersion}-darwin-amd64.tar.gz";
          sha256 = "925c4097eae9e035b0b052a66d0a149f861e2ab611a4e677c7ffd2d4e05b9b89";
        };
        "aarch64-darwin" = {
          fileName = "golangci-lint-${appReleaseVersion}-darwin-arm64.tar.gz";
          sha256 = "7b33fb1be2f26b7e3d1f3c10ce9b2b5ce6d13bb1d8468a4b2ba794f05b4445e1";
        };
      };
      supportedSystems = builtins.attrNames appReleaseBinaries;
    in
    utils.lib.eachSystem supportedSystems (system:
    let
      pkgs = nixpkgs.legacyPackages.${system};
      appReleaseBinary = appReleaseBinaries.${system};
    in
    rec {
      packages.golangci-lint = pkgs.stdenv.mkDerivation {
        pname = "golangci-lint";
        version = appReleaseVersion;

        src = pkgs.fetchurl {
          url = "https://github.com/golangci/golangci-lint/releases/download/v${appReleaseVersion}/${appReleaseBinary.fileName}";
          sha256 = appReleaseBinary.sha256;
        };

        sourceRoot = ".";

        installPhase = ''
          install -m755 -D */golangci-lint $out/bin/golangci-lint
        '';
      };
      packages.default = packages.golangci-lint;

      apps.golangci-lint = utils.lib.mkApp {
        drv = packages.golangci-lint;
      };
      apps.default = apps.golangci-lint;
    });
}
