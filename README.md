# 3Default

3Default is a collaborative engineering platform for managing and reviewing 3D CAD projects with Git-inspired version-control concepts.

The goal is to make engineering design history easier to understand through explicit revisions, parallel branches, intentional merges, and browser-based 3D review.

> **The original CAD file is the source of truth. Derived previews and converted formats are reproducible artifacts, not authoritative engineering data.**

This repository is a ground-up reconstruction of an earlier 3Default MVP. It is being developed first as a strong software-engineering portfolio project while keeping the architecture practical enough to evolve into a real product without requiring a major rewrite.

---

## Project Status

**Current phase: backend foundation and authentication**

Implemented foundations include:

* Docker-based development and VS Code Dev Container support
* Go backend and Vue 3 + TypeScript frontend
* PostgreSQL persistence with Tern migrations and sqlc
* OpenAPI-first HTTP contracts
* health and readiness endpoints
* project persistence and authenticated project creation
* PostgreSQL-backed server-side sessions
* secure opaque session tokens and SHA-256 token hashing
* secure host-only session cookies and same-origin protection
* Argon2id password hashing
* password credential persistence
* atomic user + credential creation
* unit and PostgreSQL integration tests

The next active milestone is completing user registration and login.

---

## Why 3Default?

CAD projects often evolve through many design iterations and can contain multiple related files.

Traditional file storage makes it difficult to answer questions such as:

* Which revision is current?
* What changed between versions?
* How should parallel design work be combined?
* Which revision belongs to an assembly?
* Which files are authoritative versus generated?

3Default adapts useful ideas from software version control to engineering workflows without pretending CAD files behave like source code.

The intended model includes explicit revision history, branches for parallel work, user-controlled merges, immutable historical revisions, exact revision references, authoritative source CAD files, disposable derived previews, and browser-based 3D review.

The CAD application remains responsible for editing the design itself.

---

## Architecture

3Default is being built as a **modular monolith**.

This keeps deployment and operations simple while preserving boundaries that could later be separated if product scale requires it.

```text
Browser
   │
   │ same-origin HTTP
   ▼
Vue application
   │
   │ /api/*
   ▼
Go HTTP server
   │
   ├── OpenAPI transport
   ├── authentication/session handling
   ├── application services
   └── sqlc data access
          │
          ▼
      PostgreSQL
```

During development, Vite proxies `/api/*` requests to the Go service.

The intended production shape is:

```text
Browser
   │
   ▼
Go application
   ├── serves built Vue assets
   └── serves /api/*
          │
          ▼
      PostgreSQL
```

This keeps the frontend and API on the same origin and avoids unnecessary CORS complexity.

> **As simple as it can be, but as complex as it needs to be.**

---

## Technology Stack

### Backend

* Go 1.27.1
* standard `net/http`
* PostgreSQL
* pgx
* sqlc
* Tern
* OpenAPI 3.1
* oapi-codegen
* Argon2id

### Frontend

* Vue 3
* TypeScript
* Vue Router
* Pinia
* Vite
* Vitest
* Prettier
* Oxlint
* ESLint

### Development

* Docker Desktop
* Docker Compose
* VS Code Dev Containers
* Git
* GitHub

Go is intentionally **not required on the Windows host**. Go tooling, `gopls`, builds, formatting, generators, and tests run inside Linux containers.

---

## Repository Structure

```text
3Default/
├── .devcontainer/
├── .vscode/
├── api/
├── cmd/3default/
├── db/
│   ├── migrations/
│   └── queries/
├── internal/
│   ├── api/
│   ├── auth/
│   ├── config/
│   ├── database/
│   ├── httpapi/
│   └── projects/
├── web/
├── compose.yaml
├── go.mod
├── go.sum
└── sqlc.yaml
```

Generated OpenAPI and sqlc Go files are committed as part of the reproducible project source and should not be edited manually.

---

## Authentication Model

Authentication uses PostgreSQL-backed server-side sessions.

The browser receives a cryptographically random opaque token. Only a SHA-256 hash of that token is stored in PostgreSQL.

```text
Browser
   │ opaque session token
   ▼
Secure HttpOnly cookie
   │
   ▼
SHA-256
   │
   ▼
PostgreSQL session lookup
```

Current security includes server-side session persistence, expiration and revocation, `HttpOnly`, `Secure`, `SameSite=Lax`, `__Host-` cookie naming, and same-origin request protection.

Passwords are hashed with Argon2id and stored separately from user identity records.

User creation and password-credential creation are performed atomically so failed registration cannot leave a partially created account.

Registration and login HTTP flows are still under development.

---

## Projects and API

Projects are private by default and currently include an ID, owner user ID, name, optional description, visibility, and timestamps.

For authenticated project creation, ownership is derived from the server-side session rather than accepted from the request body.

```text
POST /api/projects
        │
        ├── validate same-origin request
        ├── resolve session
        ├── identify authenticated user
        └── create project owned by that user
```

The API contract is defined in `api/openapi.yaml` and is treated as the source of truth for HTTP request and response structures.

Currently implemented endpoints:

```text
GET  /api/health
GET  /api/ready
POST /api/projects
```

---

## Development Environment

### Requirements

* Windows 11
* Docker Desktop
* Visual Studio Code
* Microsoft Dev Containers extension

Go does not need to be installed directly on Windows.

Open the repository in VS Code and run:

```text
Dev Containers: Reopen in Container
```

The development container provides Go 1.27.1, `gopls`, Linux Go tooling, shared Go caches, PostgreSQL access, and the repository mounted at `/workspace`.

Verify:

```bash
pwd
go version
```

Expected workspace:

```text
/workspace
```

---

## Running the Application

From the repository root:

```bash
docker compose up -d db api web
```

Development services:

* Go API: `http://localhost:8080`
* Vue/Vite: `http://localhost:5173`

For browser development, open [http://localhost:5173](http://localhost:5173).

Vite proxies `/api/*` requests to the Go API service.

---

## Database and Code Generation

Database migrations live in `db/migrations/`.

Check migration status:

```bash
go tool tern status \
  --migrations db/migrations \
  --conn-string "$DATABASE_URL"
```

Apply pending migrations:

```bash
go tool tern migrate \
  --migrations db/migrations \
  --conn-string "$DATABASE_URL"
```

SQL queries live in `db/queries/`.

Generate sqlc output:

```powershell
docker compose run --rm sqlc generate
```

Generate the OpenAPI Go layer from the Dev Container:

```bash
go tool oapi-codegen \
  --config api/oapi-codegen.yaml \
  api/openapi.yaml
```

Then format:

```bash
gofmt -w internal/api/openapi.gen.go
```

Generated output is checked for reproducibility before committing.

---

## Testing

Run the normal backend suite:

```bash
go test ./cmd/... ./internal/...
```

For an uncached run:

```bash
go test -count=1 ./cmd/... ./internal/...
```

Run PostgreSQL integration tests:

```bash
go test -count=1 -tags=integration -v ./internal/database
```

Current integration coverage includes user and project persistence, session lifecycle behavior, password credential persistence, database constraints, transaction rollback, and atomic user + credential creation.

Before committing:

```bash
git diff --check
```

After staging:

```bash
git diff --cached --check
```

Both should produce no output.

---

## Development Workflow

Development follows a milestone-based Git workflow.

`main` represents stable project history. Substantial work is developed on milestone branches such as:

```text
milestone/authentication
milestone/project-versioning
milestone/file-storage
```

Within each milestone, work remains divided into small logical commits. Completed milestones are merged into `main` through pull requests.

The goal is to keep both the codebase and Git history understandable as the project grows.

---

## Engineering Principles

* **Original engineering data is authoritative.** Native CAD files are the source of truth; generated previews are derived artifacts.
* **History should be immutable.** Historical revisions should not be rewritten.
* **Persistence should be atomic where correctness requires it.** Partial account creation should never remain after registration failure.
* **Identity comes from authentication.** Clients should not declare ownership of authenticated resources.
* **Infrastructure stays simple until complexity is justified.** PostgreSQL and a modular monolith are preferred over premature distributed services.
* **Development should resemble production.** The development architecture intentionally follows the expected production shape.

---

## Roadmap

### Foundation

* [x] Go backend and Vue frontend foundations
* [x] Docker Compose and Dev Container
* [x] PostgreSQL, Tern, and sqlc
* [x] OpenAPI-first API
* [x] health and readiness endpoints

### Authentication

* [x] server-side session infrastructure
* [x] secure session-token handling and cookies
* [x] same-origin request protection
* [x] Argon2id password hashing
* [x] password credential persistence
* [x] atomic registration persistence
* [ ] registration application service
* [ ] registration HTTP endpoint
* [ ] login and logout flows
* [ ] authenticated-user endpoint

### Projects

* [x] project persistence
* [x] project application service
* [x] authenticated project creation
* [ ] project listing and details
* [ ] metadata updates

### Versioning, Storage, and CAD

* [ ] revisions and branches
* [ ] immutable history and merge workflow
* [ ] conflict-resolution model
* [ ] content-addressed storage
* [ ] project and revision file references
* [ ] source CAD upload
* [ ] conversion jobs
* [ ] GLB/glTF preview pipeline
* [ ] Three.js browser viewer
* [ ] assembly/revision relationships

### Product Evolution

* [ ] public project sharing
* [ ] collaboration and permissions
* [ ] production deployment
* [ ] monitoring
* [ ] worker separation when justified

---

## Background

The original 3Default MVP explored collaborative CAD repositories, engineering versioning workflows, browser-based 3D visualization, CAD conversion, backend APIs, and product discovery.

This reconstruction focuses on architecture, maintainability, testability, security, deployment simplicity, development reproducibility, and long-term product viability.

The rebuild is intentionally incremental rather than attempting to recreate the previous system all at once.

---

## Portfolio and Product Direction

As a portfolio project, 3Default demonstrates practical engineering across Go, PostgreSQL, SQL and schema design, REST APIs, authentication, Docker, generated code workflows, integration testing, Git, and product-oriented architecture.

As a potential product, the goal is to preserve a foundation that can evolve into a usable engineering collaboration platform without discarding the portfolio implementation and starting over.

---

## License

This repository currently has no open-source license.

No license to use, modify, or redistribute the source code is granted except as otherwise permitted by applicable law or GitHub's Terms of Service.

---

## Author

**Angel Aviles**

Software engineer based in Waterloo, Ontario.

GitHub: [github.com/AngelAvilesSil](https://github.com/AngelAvilesSil)
