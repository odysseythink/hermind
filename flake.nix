{
  description = "hermind — Go port of the hermes AI agent framework";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        version = "0.1.0";
      in {
        packages.default = pkgs.buildGoModule {
          pname = "hermind";
          inherit version;
          src = ./.;
          # vendorHash should be set via `nix hash path` after the
          # first build — the lock file below is the placeholder.
          vendorHash = null;
          subPackages = [ "cmd/hermind" ];
          ldflags = [
            "-s"
            "-w"
            "-X main.Version=${version}"
            "-X main.Commit=nix"
            "-X main.BuildDate=nix"
          ];
          meta = {
            description = "Hermind — Go port of the hermes AI agent framework";
            license = pkgs.lib.licenses.mit;
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_25
            pkgs.gopls
            pkgs.golangci-lint
          ];
        };
      });
}
