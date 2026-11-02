# Contributing to ME Lab

Thank you for your interest in contributing to ME Lab! This guide will help you get started.

## Prerequisites

- **Go 1.22+** — [Install Go](https://go.dev/doc/install)
- **Node.js 20+** — [Install Node.js](https://nodejs.org/)
- **Docker** — [Install Docker](https://docs.docker.com/get-docker/)
- **Make** — Usually pre-installed on Linux/macOS; on Windows use WSL or `choco install make`


### NixOS / Nix Flakes + `devenv`
On NixOS (or any system with Nix + `devenv` installed), all Go, Node.js, SQLite, `goose`, and `staticcheck` dependencies are provisioned declaratively:
```bash
direnv allow .   # activates devenv automatically via .envrc
# or run `devenv shell` / `nix develop`
```
## Local Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/felipinhobgtom/me-lab.git
   cd me-lab
   ```

2. **Copy environment variables:**
   ```bash
   cp .env.example .env
   ```

3. **Install Go dependencies:**
   ```bash
   go mod download
   ```

4. **Install frontend dependencies:**
   ```bash
   cd web && npm ci && cd ..
   ```

5. **Run database migrations:**
   ```bash
   make migrate
   ```

6. **Start the development servers:**
   ```bash
   # Terminal 1 — API
   make dev

   # Terminal 2 — Frontend
   make dev-frontend
   ```

The API runs on `http://localhost:8080` and the frontend on `http://localhost:5173`.

## Development Commands

| Command              | Description                          |
| -------------------- | ------------------------------------ |
| `make dev`           | Run API with hot reload              |
| `make dev-frontend`  | Run frontend dev server              |
| `make build`         | Build Go binary                      |
| `make build-frontend`| Build frontend production bundle     |
| `make test`          | Run Go tests with coverage           |
| `make test-frontend` | Run frontend tests                   |
| `make test-all`      | Run all tests                        |
| `make lint`          | Run go vet + staticcheck             |
| `make lint-frontend` | Run eslint + tsc type-check          |
| `make fmt`           | Format Go code with gofmt            |
| `make migrate`       | Run database migrations              |
| `make docker-up`     | Start all services via Docker Compose|
| `make docker-down`   | Stop Docker Compose services         |
| `make clean`         | Remove build artifacts               |
| `make help`          | Show all available targets           |

## Code Style

### Go
- All code must pass `gofmt` — run `make fmt` before committing.
- All code must pass `go vet` and `staticcheck` — run `make lint`.
- Follow [Effective Go](https://go.dev/doc/effective_go) conventions.
- Export only what needs to be exported.
- Write table-driven tests where appropriate.

### TypeScript / React
- All code must pass ESLint — run `make lint-frontend`.
- Use functional components with hooks.
- Prefer named exports.
- Use TypeScript strict mode; avoid `any`.

## Testing

- **Go tests:** `make test` — runs all Go tests with race detection and generates a coverage report.
- **Frontend tests:** `make test-frontend` — runs Vitest for the React frontend.
- **All tests:** `make test-all` — runs both backend and frontend test suites.

Aim for meaningful test coverage. All new features and bug fixes should include tests.

## Commit Message Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat` — A new feature
- `fix` — A bug fix
- `docs` — Documentation changes
- `style` — Code style changes (formatting, semicolons, etc.)
- `refactor` — Code refactoring (no feature or fix)
- `perf` — Performance improvements
- `test` — Adding or updating tests
- `build` — Build system or dependency changes
- `ci` — CI/CD configuration changes
- `chore` — Other changes that don't modify src or test files

**Scopes:** `api`, `web`, `sim`, `db`, `auth`, `docs`, `docker`

**Examples:**
```
feat(sim): add channel allocation algorithm
fix(api): handle missing project ID in export endpoint
docs: update architecture diagrams
ci: add frontend lint step to workflow
```

## Pull Request Process

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```

2. **Make your changes** and commit using conventional commits.

3. **Ensure all checks pass:**
   ```bash
   make fmt
   make lint
   make lint-frontend
   make test-all
   ```

4. **Push your branch** and open a Pull Request against `main`.

5. **Fill out the PR template** — describe what changed and why.

6. **Address review feedback** — push additional commits to your branch.

7. **Merge** — PRs are squash-merged after approval. The maintainer will merge.

## Questions?

Open an issue or start a discussion if you have questions about contributing.
