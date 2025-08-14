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

          govulncheck = pkgs.govulncheck.override { buildGo124Module = buildGo125Module; };

          golangci-lint = buildGo125Module rec {
            name = "golangci-lint";
            version = "2.4.0";
            src = fetchFromGitHub {
              owner = "golangci";
              repo = "golangci-lint";
              rev = "v${version}";
              hash = "sha256-JMFSYT9aiBdr/lOy+GYigbpMHETTQAomGZ7ehyr8U/M=";
            };
            subPackages = [ "cmd/golangci-lint" ];
            vendorHash = "sha256-o01naYSkPpsXSvFlphGqJR14j3IBmTGBHpsu7DUE1Xg=";
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
              "-X main.commit=v${version}"
              "-X main.date=19700101-00:00:00"
            ];
          };
        };

        formatter = alejandra;
      }
    );
}
