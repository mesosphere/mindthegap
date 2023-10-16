# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

{
  description = "Update outdated Go dependencies interactively";

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
      appReleaseVersion = "0.9.1";
      appReleaseBinaries = {
        "x86_64-linux" = {
          fileName = "go-mod-upgrade_${appReleaseVersion}_Linux_x86_64.tar.gz";
          sha256 = "38b7f36b275fa08bedf0e4c7fb1eaf256fa632a7489abe7c40a1d2b87a688b01";
        };
        "x86_64-darwin" = {
          fileName = "go-mod-upgrade_${appReleaseVersion}_Darwin_x86_64.tar.gz";
          sha256 = "e1e0294040cfadde0f119590f37fbff73654abc482ac60c1e3ca60b867326713";
        };
        "aarch64-darwin" = {
          fileName = "go-mod-upgrade_${appReleaseVersion}_Darwin_arm64.tar.gz";
          sha256 = "15027f435a85f31346fd0796977180c43c737b7fe7bbb4fc3bcc5f4b8f32804c";
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
      packages.go-mod-upgrade = pkgs.stdenv.mkDerivation {
        pname = "go-mod-upgrade";
        version = appReleaseVersion;

        src = pkgs.fetchurl {
          url = "https://github.com/oligot/go-mod-upgrade/releases/download/v${appReleaseVersion}/${appReleaseBinary.fileName}";
          sha256 = appReleaseBinary.sha256;
        };

        sourceRoot = ".";

        installPhase = ''
          install -m755 -D go-mod-upgrade $out/bin/go-mod-upgrade
        '';
      };
      packages.default = packages.go-mod-upgrade;

      apps.go-mod-upgrade = utils.lib.mkApp {
        drv = packages.go-mod-upgrade;
      };
      apps.default = apps.go-mod-upgrade;
    });
}
