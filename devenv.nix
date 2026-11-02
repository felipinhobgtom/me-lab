{ pkgs, lib, config, ... }:

{
  env = {
    CGO_ENABLED = "0";
    PORT = "8080";
    DATABASE_URL = "${config.devenv.state}/me-lab.db";
    JWT_SECRET = "dev-local-devenv-secret-do-not-use-in-prod";
    LOG_LEVEL = "debug";
  };

  languages.go = {
    enable = true;
  };

  languages.javascript = {
    enable = true;
    npm.enable = true;
  };

  packages = with pkgs; [
    go-tools # staticcheck
    goose
    sqlite
    jq
    curl
    azure-cli
  ];

  processes = {
    api.exec = "go run ./cmd/api";
    web.exec = "npm --prefix web run dev";
  };

  scripts = {
    melab-test.exec = ''
      go test -race ./...
      npm --prefix web run test
    '';
    melab-lint.exec = ''
      go vet ./...
      npm --prefix web run type-check
    '';
  };

  enterShell = ''
    echo "⚡ ME Lab devenv environment ready"
    echo "   Commands: devenv up | make dev | make test-all | melab-test"
  '';

  enterTest = ''
    go vet ./...
    go test -race ./...
  '';
}
