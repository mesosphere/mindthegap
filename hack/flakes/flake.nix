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
          golangci-lint = pkgs.golangci-lint.override { buildGoModule = buildGo123Module; };

          govulncheck  = pkgs.govulncheck.override { buildGoModule = buildGo123Module; };

          go-mod-upgrade = buildGo123Module rec {
            name = "go-mod-upgrade";
            version = "0.10.0";
            src = fetchFromGitHub {
              owner = "oligot";
              repo = "go-mod-upgrade";
              rev = "v${version}";
              hash = "sha256-BuHyqv0rK1giNiPO+eCx13rJ9L6y2oCDdKW1sJXyFg4=";
            };
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = "sha256-Qx+8DfeZyNSTf5k4juX7+0IXT4zY2LJMuMw3e1HrxBs=";
            ldflags = [ "-s" "-w" "-X" "main.version=v${version}" ];
          };
        };

        formatter = alejandra;
      }
    );
}
