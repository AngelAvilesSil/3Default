# 3Default

3Default is a collaborative engineering platform for managing and reviewing 3D CAD projects with Git-inspired version-control concepts.

The goal is to make engineering design history easier to understand through explicit revisions, parallel branches, intentional merges, and browser-based 3D review.

> **The original CAD file is the source of truth. Derived previews and converted formats are reproducible artifacts, not authoritative engineering data.**

This repository is a ground-up reconstruction of an earlier 3Default MVP. It is being developed first as a strong software-engineering portfolio project while keeping the architecture practical enough to evolve into a real product without requiring a major rewrite.

---

## Project Status

**Current phase: project versioning and authenticated project APIs**

Implemented foundations include:

* Docker-based development and VS Code Dev Container support
* Go backend and Vue 3 + TypeScript frontend
* PostgreSQL persistence with Tern migrations and sqlc
* OpenAPI-first HTTP contracts
* health and readiness endpoints
* project persistence and authenticated project creation, listing, detail reads, and metadata updates
* project revision and branch persistence with automatic `main` branch creation
* atomic revision creation with optimistic branch-head updates
* authenticated branch creation, branch listing and history traversal, revision reads, and revision creation
* atomic user registration and password-credential creation
* password creation policy and local weak-password screening
* Argon2id password hashing
* PostgreSQL-backed server-side sessions
* secure opaque session tokens and SHA-256 token hashing
* secure host-only session cookies
* login, logout, and authenticated-user HTTP flows
* unsafe cross-origin browser request protection
* unit and PostgreSQL integration tests

The authentication milestone and basic authenticated project operations are complete. The backend now includes the first project-versioning API surface: branch listing and creation, project-scoped revision reads, atomic revision creation with optimistic branch-head concurrency, and branch-head history traversal. Branch rename/delete, immutable-history hardening and cycle prevention, storage integration, and CAD workflows remain future work.

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
│   ├── projects/
│   └── versioning/
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

Registration creates the user and password credential atomically and deliberately does **not** create a session. Login is a separate operation that verifies credentials and creates the authenticated session.

```text
POST /api/auth/register
        │
        └── create user + password credential
            without creating a session

POST /api/auth/login
        │
        ├── verify credentials
        └── create PostgreSQL session
                │
                └── __Host-3default_session cookie

GET /api/auth/me
        │
        ├── resolve active session
        └── return authenticated user

POST /api/auth/logout
        │
        ├── revoke matching server-side session
        └── expire browser cookie
```

Each session starts with 32 cryptographically random bytes. The opaque token is sent to the browser, while only its SHA-256 hash is stored in PostgreSQL. Sessions currently expire after seven days.

The browser cookie is named `__Host-3default_session` and uses `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, and no `Domain` attribute, keeping it host-only.

Passwords are normalized to Unicode NFC before creation-policy validation. New passwords must contain 15–128 Unicode code points and must not match the local password blocklist.

Passwords are hashed with Argon2id using:

* 19 MiB of memory
* 2 iterations
* parallelism of 1
* a 16-byte random salt
* a 32-byte derived key

Login uses the same invalid-credentials response for an unknown account and an incorrect password. Missing-account verification still performs password-hashing work using a dummy hash to reduce account-enumeration timing differences.

Login attempts are throttled by an in-memory limiter keyed by normalized email. Each identifier can make 5 immediate attempts and recovers 1 attempt per minute. The limiter tracks at most 10,000 identifiers per application process, evicting the least-recently-used entry when full. Because the limiter is intentionally in-memory, its state resets when the application process restarts.

Logout is idempotent from the client's perspective: a missing, invalid, expired, or already-revoked session is treated as already logged out, while successful logout expires the browser cookie.

Unsafe cross-origin browser requests are rejected by the HTTP layer. The detailed request and response contract remains defined in `api/openapi.yaml`.

---

## Projects and API

Projects are private by default and currently include an ID, owner user ID, name, optional description, visibility, and timestamps.

For authenticated project operations, ownership is derived from the server-side session rather than accepted from the request.

```text
POST /api/projects
        │
        ├── validate cross-origin request safety
        ├── resolve session
        ├── identify authenticated user
        └── create project owned by that user

GET /api/projects
        │
        ├── resolve session
        └── list projects owned by the authenticated user

GET /api/projects/{projectId}
        │
        ├── resolve session
        └── read the project only when it belongs to
            the authenticated user

PATCH /api/projects/{projectId}
        │
        ├── validate cross-origin request safety
        ├── resolve session
        └── update metadata only when the project belongs to
            the authenticated user
```

Project detail reads and metadata updates are owner-scoped at the persistence boundary. A project that does not exist and a project owned by another user both return `404`, so these endpoints do not reveal whether another user's project exists. Public-project reads are not implemented yet, even though projects already carry a visibility field.

Project metadata updates currently support `name` and `description`. Omitted fields remain unchanged. A null or blank `name` is invalid, while a null or blank `description` clears the description. An empty update is invalid, and a successful update refreshes the project's `updated_at` timestamp. Project visibility is not editable yet.

The API contract is defined in `api/openapi.yaml` and is treated as the source of truth for HTTP request and response structures.

Currently implemented endpoints:

```text
GET  /api/health
GET  /api/ready

POST /api/auth/register
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/me

GET   /api/projects
GET   /api/projects/{projectId}
POST  /api/projects
PATCH /api/projects/{projectId}

GET   /api/projects/{projectId}/branches
POST  /api/projects/{projectId}/branches
GET   /api/projects/{projectId}/branches/{branchId}/history
GET   /api/projects/{projectId}/revisions/{revisionId}
POST  /api/projects/{projectId}/branches/{branchId}/revisions
```

---

## Versioning Model and API

Each project is created transactionally with a `main` branch. Revisions belong to the project rather than to a branch. A branch is a named, movable pointer to a revision head, and a newly created project's `main` branch initially has no head.

The current revision graph supports zero, one, or two parents:

```text
Root revision
    parent = null
    merge parent = null

Normal revision
    parent = previously observed target-branch head
    merge parent = null

Merge revision
    parent = previously observed target-branch head
    merge parent = a distinct second revision
```

For a merge revision, both parents must belong to the same project, the merge parent must differ from the primary parent, and a merge parent cannot be supplied when the target branch has no expected head. The current merge model records revision ancestry only; 3Default does not yet perform automatic CAD-content merging or conflict resolution.

Revision creation and target-branch advancement happen atomically in one database transaction. The client must provide `expectedHeadRevisionId` as an optimistic-concurrency precondition:

* `null` means the caller explicitly observed an empty branch.
* a UUID means the caller observed that revision as the branch head.
* omitting the field is invalid.

The branch head is advanced only if it still matches the caller's expected value. If another revision has already moved the branch head, the candidate revision is rolled back and the API returns `409 Conflict`.

`mergeParentRevisionId` is optional. When omitted or `null`, the revision has only its primary parent. When supplied, it records the second parent of a merge revision.

The authenticated project owner is currently also recorded as the revision author. Project ownership is resolved from the server-side session rather than accepted from the request.

Implemented versioning endpoints are:

```text
GET  /api/projects/{projectId}/branches
POST /api/projects/{projectId}/branches
GET  /api/projects/{projectId}/branches/{branchId}/history
GET  /api/projects/{projectId}/revisions/{revisionId}
POST /api/projects/{projectId}/branches/{branchId}/revisions
```

Branch listing returns each branch and its current head revision, if any. Branch creation accepts a name and a required-but-nullable `headRevisionId`:

* `null` creates an intentionally empty branch.
* a UUID creates the branch at that exact revision in the same project.
* omitting `headRevisionId` is invalid.

Branch names are trimmed, must be nonblank, and are unique within a project. Name uniqueness is currently case-sensitive. Creating a duplicate branch name returns `409 Conflict`. A missing project or supplied head revision is exposed as `404`.

Branch creation deliberately accepts an exact revision rather than a source branch whose current head would be copied. This keeps the starting point explicit and avoids racing against a source branch that may move between observation and branch creation.

Revision reads are scoped to a project. A project that does not exist and a project owned by another user are both exposed as `404` through the owner-scoped application service.

Branch history is exposed through `GET /api/projects/{projectId}/branches/{branchId}/history`. For an existing non-empty branch, traversal starts from the branch's current persisted head and returns every unique revision reachable by recursively following both `parentRevisionId` and `mergeParentRevisionId`. An existing empty branch returns `[]`.

History results use a deterministic presentation order of `createdAt` descending and revision ID ascending. That ordering does **not** define a linear commit chain. The parent IDs on each revision are the authoritative graph structure, so branching and merge ancestry remain explicit in the response.

Historical revisions are treated as append-only by the application: there are no revision update or delete operations in the current service or HTTP API. The database schema enforces same-project parent references and several parent constraints, but it does **not** currently prevent arbitrary direct SQL updates to revision rows or fully enforce cycle prevention. Stronger immutable-history enforcement and graph validation remain future work.

Branch renaming and deletion, immutable-history hardening and cycle prevention, CAD/file references, storage, conversion, visualization, and user-facing merge/conflict-resolution workflows are not implemented yet.

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
* [x] unsafe cross-origin browser request protection
* [x] Argon2id password hashing
* [x] password credential persistence
* [x] password creation policy and local blocklist
* [x] atomic registration persistence
* [x] registration application service
* [x] registration HTTP endpoint
* [x] login flow
* [x] logout flow
* [x] authenticated-user endpoint
* [x] login-attempt rate limiting

### Projects

* [x] project persistence
* [x] project application service
* [x] authenticated project creation
* [x] project listing and details
* [x] metadata updates

### Versioning, Storage, and CAD

* [x] revision and branch persistence foundation
* [x] automatic `main` branch creation
* [x] atomic revision creation with optimistic branch-head updates
* [x] authenticated branch/revision reads and revision-creation API
* [x] authenticated branch creation API
* [ ] branch rename/delete and broader branch management
* [x] branch-head DAG traversal and history API
* [ ] immutable-history hardening and cycle prevention
* [ ] merge and conflict-resolution workflow
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
