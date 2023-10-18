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
      releaseVersion = "1.12.0";
      releaseBinaries = {
        "x86_64-linux" = {
          fileName = "mindthegap_v${releaseVersion}_linux_amd64.tar.gz";
          sha256 = "d8de421a43a16baca9f7c235c23855bc252b0d045a9dac850b3b4d7ea4822b34";
        };
        "aarch64-linux" = {
          fileName = "mindthegap_v${releaseVersion}_linux_arm64.tar.gz";
          sha256 = "9dc16c1aaaedb8034f35ab53501122f64e42e49bddbdeb9c53eea9a2701e766f";
        };
        "x86_64-darwin" = {
          fileName = "mindthegap_v${releaseVersion}_darwin_amd64.tar.gz";
          sha256 = "68fc3b57144e1804b95c5654f624a5e63b14014f61d04a8b5de39c513f5fed9a";
        };
        "aarch64-darwin" = {
          fileName = "mindthegap_v${releaseVersion}_darwin_arm64.tar.gz";
          sha256 = "0a2bd11437ee76ec4cce3b0d903363ce082e1a69ebac3301313a2cd4739e1a70";
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
