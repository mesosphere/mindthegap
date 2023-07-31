{
  description = "Easily create and use bundles for air-gapped environments ";

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
      releaseVersion = "1.11.0";
      releaseBinaries = {
        "x86_64-linux" = {
          fileName = "mindthegap_v${releaseVersion}_linux_amd64.tar.gz";
          sha256 = "0687f239413c3a69be44a2056e83a5520e71de41a4633eea73ab485ed062cbe0";
        };
        "aarch64-linux" = {
          fileName = "mindthegap_v${releaseVersion}_linux_arm64.tar.gz";
          sha256 = "7936510e2234612fb1acd0c428b37806e411680e3bccacad0cd001da83592af0";
        };
        "x86_64-darwin" = {
          fileName = "mindthegap_v${releaseVersion}_darwin_amd64.tar.gz";
          sha256 = "d6eccda3abf113c4e019aeab3ee8887bbdc6045f630ffaf3a881915d9212a2b2";
        };
        "aarch64-darwin" = {
          fileName = "mindthegap_v${releaseVersion}_darwin_arm64.tar.gz";
          sha256 = "b9e794e8ca292bb95d18dfdb6f935263c23a960a7b2a70b6d10c0b5a6a5fd4bd";
        };
      };
      supportedSystems = builtins.attrNames releaseBinaries;
    in
    utils.lib.eachSystem supportedSystems (system:
    let
      pkgs = nixpkgs.legacyPackages.${system};
      releaseBinary = releaseBinaries.${system};
    in
    rec {
      packages.mindthegap = pkgs.stdenv.mkDerivation {
        pname = "mindthegap";
        version = releaseVersion;

        src = pkgs.fetchurl {
          url = "https://github.com/mesosphere/mindthegap/releases/download/v${releaseVersion}/${releaseBinary.fileName}";
          sha256 = releaseBinary.sha256;
        };

        sourceRoot = ".";

        installPhase = ''
          install -m755 -D mindthegap $out/bin/mindthegap
        '';
      };
      packages.default = packages.mindthegap;

      apps.mindthegap = utils.lib.mkApp {
        drv = packages.mindthegap;
      };
      apps.default = apps.mindthegap;
    });
}
