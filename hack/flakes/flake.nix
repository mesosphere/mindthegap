{
  description = "Useful flakes for golang and Kubernetes projects";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = inputs @ { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      with nixpkgs.legacyPackages.${system}; rec {
        packages = rec {
          gotestsum = pkgs.gotestsum.override { buildGoModule = buildGo125Module; };
        };

        formatter = alejandra;
      }
    );
}
