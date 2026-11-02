{
  description = "ME Lab — Applied Energistics 2 Network Designer & Simulator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            name = "me-lab-dev";
            packages = with pkgs; [
              go
              gopls
              gotools
              go-tools # staticcheck
              goose
              nodejs_22
              sqlite
              jq
              curl
              git
              docker-compose
            ];

            env = {
              PORT = "8080";
              DATABASE_URL = "me-lab.db";
              LOG_LEVEL = "debug";
            };

            shellHook = ''
              echo "⚡ ME Lab Nix devShell active (Go $(go version | awk '{print $3}'), Node $(node --version), SQLite $(sqlite3 --version | awk '{print $1}'))"
            '';
          };
        }
      );
    };
}
