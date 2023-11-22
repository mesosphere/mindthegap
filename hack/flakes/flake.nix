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
          golangci-lint = pkgs.golangci-lint.override { buildGoModule = buildGo121Module; };

          go-mod-upgrade = buildGo121Module rec {
            name = "go-mod-upgrade";
            version = "0.9.1";
            src = fetchFromGitHub {
              owner = "oligot";
              repo = "go-mod-upgrade";
              rev = "v${version}";
              hash = "sha256-+C0IMb7MU1fq/P0/tTUNmzznZ1q5M69491pO5yBZlVs=";
            };
            doCheck = false;
            subPackages = [ "." ];
            vendorHash = "sha256-8rbRxtOiKmnf68kjsUCXaZf+MHI1n5aXa91Aneq9SKo=";
            ldflags = [ "-s" "-w" "-X" "main.version=v${version}" ];
          };
        };

        formatter = alejandra;
      }
    );
}
