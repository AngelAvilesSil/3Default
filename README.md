# 3Default

3Default is a collaborative engineering platform for managing and reviewing 3D CAD projects with Git-inspired version-control concepts.

The goal is to make engineering design history easier to understand through explicit revisions, parallel branches, intentional merges, and browser-based 3D review.

> **The original CAD file is the source of truth. Derived previews and converted formats are reproducible artifacts, not authoritative engineering data.**

This repository is a ground-up reconstruction of an earlier 3Default MVP. It is being developed first as a strong software-engineering portfolio project while keeping the architecture practical enough to evolve into a real product without requiring a major rewrite.

---

## Project Status

**Current phase: project versioning, authenticated content storage, and a durable GLB preview pipeline**

Implemented foundations include:

* Docker-based development and VS Code Dev Container support
* Go backend and Vue 3 + TypeScript frontend
* PostgreSQL persistence with Tern migrations and sqlc
* OpenAPI-first HTTP contracts
* health and readiness endpoints
* project persistence and authenticated project creation, listing, detail reads, and metadata updates
* project revision and branch persistence with automatic `main` branch creation
* atomic revision creation with optimistic branch-head updates
* authenticated branch creation, rename, and deletion, branch listing and history traversal, revision reads, and revision creation
* immutable SHA-256 content-addressed filesystem storage with physical deduplication
* PostgreSQL project-file metadata and exact revision-file snapshot persistence
* authenticated multipart source-file upload with project ownership checked before physical storage
* authenticated project-file metadata listing and detail reads
* authenticated streaming source-file download with integrity verification
* authenticated exact revision-file snapshot creation and listing
* durable PostgreSQL-backed CAD conversion jobs with `pending`, `running`, `succeeded`, and `failed` states
* authenticated conversion-job creation, project-file job history, and project-scoped job detail APIs
* embedded conversion worker started by the Go application, with PostgreSQL-backed claiming, at-least-once execution semantics, graceful requeue, and startup recovery
* immutable derived-preview persistence using the shared content-addressed storage layer and transactional conversion-output metadata
* GLB 2.0 pass-through converter adapter with structural validation and a replaceable converter boundary for future native CAD conversion
* authenticated latest-successful GLB preview streaming through `/api/projects/{projectId}/files/{projectFileId}/preview`
* persistent Docker storage configured through `STORAGE_ROOT`
* atomic user registration and password-credential creation
* password creation policy and local weak-password screening
* Argon2id password hashing
* PostgreSQL-backed server-side sessions
* secure opaque session tokens and SHA-256 token hashing
* secure host-only session cookies
* login, logout, and authenticated-user HTTP flows
* unsafe cross-origin browser request protection
* unit and PostgreSQL integration tests

The authentication, core project-versioning, file-storage, durable conversion-job, and first GLB preview-pipeline foundations are now implemented. The backend supports branch listing, creation, rename, and deletion, project-scoped revision reads, atomic revision creation with optimistic branch-head concurrency, branch-head history traversal, immutable content-addressed source-file storage, owner-scoped project-file upload, metadata reads, streaming source-file download, exact revision-file snapshots supplied during revision creation, authenticated revision-file snapshot reads, durable conversion-job creation and inspection, persisted derived preview outputs, and an embedded worker that is started by `main` and finalizes successful jobs only together with their output metadata. The current converter is intentionally limited to structurally valid GLB 2.0 pass-through: it does not yet convert native CAD or STEP files into GLB. Three.js browser visualization, real CAD-to-GLB conversion, immutable-history hardening, and user-facing merge/conflict-resolution workflows remain future work.

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
   ├── embedded conversion worker
   │      └── replaceable converter adapter
   ├── sqlc data access
   │      │
   │      ▼
   │  PostgreSQL
   │
   └── immutable content storage
          │
          ▼
      STORAGE_ROOT filesystem
```

During development, Vite proxies `/api/*` requests to the Go service.

The intended production shape is:

```text
Browser
   │
   ▼
Go application
   ├── serves built Vue assets
   ├── serves /api/*
   ├── persists application metadata, durable jobs, and derived-output metadata in PostgreSQL
   ├── runs the embedded conversion worker with the configured converter adapter
   └── stores immutable source and derived preview content through the
       configured content-storage implementation
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
│   ├── conversionjobs/
│   ├── database/
│   ├── filestorage/
│   ├── httpapi/
│   ├── projects/
│   ├── storage/
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

GET  /api/projects/{projectId}/files
POST /api/projects/{projectId}/files
GET  /api/projects/{projectId}/files/{projectFileId}
GET  /api/projects/{projectId}/files/{projectFileId}/content
GET  /api/projects/{projectId}/files/{projectFileId}/conversion-jobs
POST /api/projects/{projectId}/files/{projectFileId}/conversion-jobs
GET  /api/projects/{projectId}/conversion-jobs/{conversionJobId}

GET    /api/projects/{projectId}/branches
POST   /api/projects/{projectId}/branches
PATCH  /api/projects/{projectId}/branches/{branchId}
DELETE /api/projects/{projectId}/branches/{branchId}
GET    /api/projects/{projectId}/branches/{branchId}/history
GET    /api/projects/{projectId}/revisions/{revisionId}
GET    /api/projects/{projectId}/revisions/{revisionId}/files
POST   /api/projects/{projectId}/branches/{branchId}/revisions
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

Revision creation, exact revision-file references, and target-branch advancement happen atomically in one database transaction. The client must provide `expectedHeadRevisionId` as an optimistic-concurrency precondition:

* `null` means the caller explicitly observed an empty branch.
* a UUID means the caller observed that revision as the branch head.
* omitting the field is invalid.

Revision creation also requires `projectFileIds`, which represents the complete project-file snapshot for the new revision:

* `[]` means the caller explicitly creates an empty file snapshot.
* a list of UUIDs means exactly those project files belong to the revision snapshot.
* omitting `projectFileIds` or supplying `null` is invalid at the HTTP API.
* duplicate IDs and the nil UUID are invalid, and every referenced file must exist in the same project.

The snapshot is not a delta from the primary parent and no files are inherited implicitly. There are no mutable attach or detach endpoints; after creation, the application and HTTP API treat the revision-file snapshot as immutable.

`GET /api/projects/{projectId}/revisions/{revisionId}/files` returns the project-file metadata referenced by that exact revision. The read is owner-scoped and verifies that the revision exists before listing its files, so a missing revision is distinguishable from a valid revision with an empty snapshot. Results use a deterministic presentation order of file creation time ascending and file ID ascending; that order has no semantic meaning within the snapshot.

The branch head is advanced only if it still matches the caller's expected value. If another revision has already moved the branch head, the candidate revision is rolled back and the API returns `409 Conflict`.

`mergeParentRevisionId` is optional. When omitted or `null`, the revision has only its primary parent. When supplied, it records the second parent of a merge revision.

The authenticated project owner is currently also recorded as the revision author. Project ownership is resolved from the server-side session rather than accepted from the request.

Implemented versioning endpoints are:

```text
GET    /api/projects/{projectId}/branches
POST   /api/projects/{projectId}/branches
PATCH  /api/projects/{projectId}/branches/{branchId}
DELETE /api/projects/{projectId}/branches/{branchId}
GET    /api/projects/{projectId}/branches/{branchId}/history
GET    /api/projects/{projectId}/revisions/{revisionId}
GET    /api/projects/{projectId}/revisions/{revisionId}/files
POST   /api/projects/{projectId}/branches/{branchId}/revisions
```

Branch listing returns each branch and its current head revision, if any. Branch creation accepts a name and a required-but-nullable `headRevisionId`:

* `null` creates an intentionally empty branch.
* a UUID creates the branch at that exact revision in the same project.
* omitting `headRevisionId` is invalid.

Branch names are trimmed, must be nonblank, and are unique within a project. Name uniqueness is currently case-sensitive. Creating a duplicate branch name returns `409 Conflict`. A missing project or supplied head revision is exposed as `404`.

Branch creation deliberately accepts an exact revision rather than a source branch whose current head would be copied. This keeps the starting point explicit and avoids racing against a source branch that may move between observation and branch creation.

Branch renaming is exposed through `PATCH /api/projects/{projectId}/branches/{branchId}` with a required `name`. The service trims the supplied name and rejects a blank result. Renaming to the branch's current exact name succeeds idempotently, while renaming to another existing branch name in the same project returns `409 Conflict`. Missing projects and missing or wrong-project branches are exposed as `404`.

A rename changes the branch name and `updatedAt` only. The branch ID, project ID, `headRevisionId`, and `createdAt` are preserved. Branch names remain case-sensitive, and there is currently no special rename restriction for the `main` branch.

Branch deletion is exposed through `DELETE /api/projects/{projectId}/branches/{branchId}`. Deletion removes only the movable branch pointer; it does not delete project revisions. Both empty and non-empty branches may be deleted, there is no special deletion protection for `main`, and deleting the project's last branch is allowed. A project can therefore temporarily have zero branches and later create a new branch either empty or at an exact existing revision.

Missing or non-owned projects and missing or wrong-project branches are exposed as `404`. Malformed project or branch IDs are rejected as `400`, unsafe cross-origin browser requests are rejected by the shared request protection, and successful deletion returns `204 No Content`. Branch deletion has no `409 Conflict` case.

Revision reads are scoped to a project. A project that does not exist and a project owned by another user are both exposed as `404` through the owner-scoped application service.

Branch history is exposed through `GET /api/projects/{projectId}/branches/{branchId}/history`. For an existing non-empty branch, traversal starts from the branch's current persisted head and returns every unique revision reachable by recursively following both `parentRevisionId` and `mergeParentRevisionId`. An existing empty branch returns `[]`.

History results use a deterministic presentation order of `createdAt` descending and revision ID ascending. That ordering does **not** define a linear commit chain. The parent IDs on each revision are the authoritative graph structure, so branching and merge ancestry remain explicit in the response.

Historical revisions are treated as append-only by the application: there are no revision update or delete operations in the current service or HTTP API. The database schema enforces same-project parent references and several parent constraints, but it does **not** currently prevent arbitrary direct SQL updates to revision rows or fully enforce cycle prevention. Stronger immutable-history enforcement and graph validation remain future work.

Immutable-history hardening and cycle prevention, broader branch management, CAD conversion, visualization, and user-facing merge/conflict-resolution workflows are not implemented yet.

---

## File Storage Model and API

Uploaded source files are stored as immutable content-addressed objects. The original uploaded source file is authoritative engineering data; converted formats and previews are disposable derived artifacts and never replace the source of truth.

Physical objects are addressed by the SHA-256 hash of their complete contents:

```text
STORAGE_ROOT/
└── objects/
    └── sha256/
        └── <first 2 hex characters>/
            └── <next 2 hex characters>/
                └── <full SHA-256 hash>
```

The filesystem store streams each upload into a temporary file while calculating its SHA-256 hash and byte size. A successfully read object is published immutably at its content-addressed path. Uploading identical bytes reuses the same physical object rather than creating another copy. Existing objects are checked for expected size and hash integrity before reuse.

PostgreSQL stores the application metadata separately:

* `content_objects` records the global SHA-256 identity and size of stored content.
* `project_files` records project-scoped metadata including the uploader, content hash, original filename, optional media type, and creation time.
* `project_revision_files` stores the exact project-file references that form each revision snapshot.
* `conversion_job_outputs` records the derived content hash, media type, and creation time produced by a successful conversion job while reusing `content_objects` for the immutable physical bytes.

The revision-file relation is now part of revision creation. The revision row, its exact project-file references, and the target branch-head update are persisted in one transaction. A missing or wrong-project file, a duplicate file ID, or a branch-head conflict rolls back the candidate revision and its file references.

Authenticated upload is exposed through:

```text
POST /api/projects/{projectId}/files
```

The endpoint accepts `multipart/form-data` with exactly one `file` part. The authenticated user is derived from the server-side session. Project ownership is verified before physical content is written, so an unauthorized or nonexistent project does not create stored bytes.

The upload path remains streaming rather than buffering the complete CAD/source file in application memory:

```text
authenticated request
        │
        ├── verify project ownership
        │
        ▼
multipart file stream
        │
        ▼
immutable filesystem store
        │
        ├── calculate SHA-256
        ├── calculate byte size
        └── publish content-addressed object
                │
                ▼
PostgreSQL metadata transaction
        │
        ├── ensure matching content_objects row
        └── create project_files row
```

Content-object metadata and project-file metadata are persisted atomically in PostgreSQL. If physical storage fails, no metadata is created. If physical storage succeeds but later metadata persistence fails, the immutable physical object may remain unreferenced; no compensating delete is attempted because the same content may already be shared by another project or concurrent upload. Eventual orphan garbage collection can be added later.

Project-file metadata can be read through:

```text
GET /api/projects/{projectId}/files
GET /api/projects/{projectId}/files/{projectFileId}
```

Both metadata operations are owner-scoped. A project owned by another user is treated the same as a nonexistent project.

The authoritative source bytes for a project file can be downloaded through:

```text
GET /api/projects/{projectId}/files/{projectFileId}/content
```

The download is authenticated and owner-scoped through the same project-file metadata lookup used by the detail API. After the metadata record is resolved, its persisted SHA-256 identifies the immutable physical content object in `STORAGE_ROOT`.

Before a content stream is returned, the filesystem store validates the requested SHA-256, requires the stored object to be a regular file, hashes the complete object, and verifies that the resulting digest matches its content-addressed identity. The verified file is then rewound and streamed to the HTTP response rather than buffered completely in application memory.

Successful downloads return `application/octet-stream`. `Content-Disposition` is generated safely from the original uploaded filename and uses an attachment disposition.

A missing or wrong-project metadata record remains a normal owner-scoped `404`. If project-file metadata exists but its physical content object is missing, non-regular, unreadable, or hash-corrupt, the condition is treated as an internal storage inconsistency and the download endpoint returns `500` rather than pretending that the project-file metadata does not exist.

The current HTTP API exposes project-file metadata, exact revision-file snapshot metadata, authoritative physical source-file download, durable conversion-job orchestration, and retrieval of the latest successful GLB preview for a project file. Derived preview bytes reuse the immutable content-addressed storage layer, while their conversion-job relationship is stored separately so source `project_files` remain authoritative. HTTP range requests, conditional caching/ETags, browser visualization, and real native-CAD-to-GLB conversion remain future work.

The filesystem root is supplied through the required `STORAGE_ROOT` environment variable. The Docker development configuration uses:

```text
STORAGE_ROOT=/var/lib/3default/storage
```

and persists that path with the `storage-data` named Docker volume, keeping uploaded engineering data outside the Git working tree.

---

## Conversion Job Model and API

Conversion jobs provide the durable orchestration layer between authoritative source files and the derived-preview pipeline.

A job belongs to exactly one project file and therefore to exactly one project. PostgreSQL stores job lifecycle state separately from derived-output metadata so execution can be retried or recovered without changing the authoritative source file or rewriting prior successful outputs.

Current states are:

```text
pending
   │
   │ worker claim
   ▼
running
   ├── converter succeeds ──► succeeded
   ├── converter fails ─────► failed
   ├── graceful shutdown ───► pending
   └── process crash ───────► recovered to pending on startup
```

The schema enforces the state/timestamp relationships, keeps `attempt_count`, stores a trimmed `last_error` only for failed jobs, and retains completed jobs as history. At most one `pending` or `running` job may exist for the same project file at a time. After a job reaches `succeeded` or `failed`, a later conversion request may create another job for that file.

The embedded worker claims the oldest pending job with PostgreSQL row locking and `FOR UPDATE SKIP LOCKED`, increments its attempt count, and transitions it to `running`. Execution is intentionally **at least once** rather than exactly once. A converter must therefore tolerate the possibility that a previously started job is executed again after process failure and startup recovery.

Graceful cancellation requeues the currently running job instead of marking it failed. Startup recovery also requeues rows left in `running` by an earlier process. This recovery model is correct for the current single-process, single-worker deployment shape. A future multi-process or distributed worker topology would require a lease or heartbeat mechanism rather than globally requeueing all running jobs.

The converter itself is behind a replaceable adapter boundary. The current implementation uses a GLB pass-through adapter: it accepts structurally valid GLB 2.0 input, validates the container framing while streaming, and returns the same GLB bytes as the derived preview with media type `model/gltf-binary`. Unsupported source formats fail the job rather than being mislabeled as converted previews.

The worker stores the derived bytes in the same immutable content-addressed filesystem used by source content. It then finalizes success in one PostgreSQL transaction that ensures the matching `content_objects` row, creates the `conversion_job_outputs` row, and transitions the job from `running` to `succeeded`. A job therefore cannot be recorded as succeeded without persisted output metadata. If physical storage succeeds but the database transaction fails, the immutable physical object may remain unreferenced and can be handled later by orphan garbage collection.

Conversion jobs are exposed through authenticated, owner-scoped endpoints:

```text
POST /api/projects/{projectId}/files/{projectFileId}/conversion-jobs
GET  /api/projects/{projectId}/files/{projectFileId}/conversion-jobs
GET  /api/projects/{projectId}/conversion-jobs/{conversionJobId}
```

`POST` creates a durable `pending` job and returns `201 Created`. If the same project file already has a `pending` or `running` job, the request returns `409 Conflict`.

The project-file history endpoint returns retained jobs newest first. The project-scoped detail endpoint returns one job by ID. Missing projects, non-owned projects, missing files, wrong-project files, missing jobs, and wrong-project jobs remain owner-scoped `404` cases rather than exposing another user's resources.

The HTTP response includes:

* job ID
* project ID
* project-file ID
* status
* attempt count
* nullable last error
* creation time
* nullable start time
* nullable finish time
* update time

All nested project routes participate in the same PostgreSQL-backed session resolution as the project collection and project-detail APIs. Unsafe cross-origin browser requests to the conversion-job creation endpoint are rejected by the shared request protection.


### GLB Preview Retrieval

The latest successful preview for a project file is exposed through:

```text
GET /api/projects/{projectId}/files/{projectFileId}/preview
```

The endpoint is authenticated and owner-scoped. The service first verifies project ownership and that the project file belongs to that project, then resolves the newest persisted output attached to a `succeeded` conversion job. Earlier successful outputs remain retained as conversion history; the endpoint selects the latest successful one rather than mutating or replacing historical rows.

Only persisted outputs with media type `model/gltf-binary` are served by this endpoint. A project or project file that is missing or not owned by the current user returns `404`, and a project file with no successful preview also returns `404`. Missing, unreadable, or integrity-invalid physical content and incompatible persisted preview media types are treated as internal inconsistencies and return `500`.

Successful responses stream the GLB body directly as `model/gltf-binary`. The response intentionally does not use `Content-Disposition: attachment`, because the resource is intended for browser/Three.js consumption rather than forced download. The generated OpenAPI transport closes the returned content stream after copying it to the response.

The conversion service, preview service, and embedded worker are wired into the HTTP application in `main`. The worker starts with the GLB pass-through adapter and can mark a job successful only through the transactional output-finalization path. This completes the first end-to-end preview pipeline while keeping the converter boundary replaceable for later STEP/native-CAD conversion.

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

The development container provides Go 1.27.1, `gopls`, Linux Go tooling, shared Go caches, PostgreSQL access, the repository mounted at `/workspace`, and persistent application storage mounted at `/var/lib/3default/storage`.

The application requires both `DATABASE_URL` and `STORAGE_ROOT`. Docker Compose and the Dev Container configure these automatically for local development.

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
* PostgreSQL data: persistent `postgres-data` Docker volume
* immutable source and derived preview content: persistent `storage-data` Docker volume mounted at `/var/lib/3default/storage`

For browser development, open [http://localhost:5173](http://localhost:5173).

Vite proxies `/api/*` requests to the Go API service.

Uploaded source content and generated preview content are intentionally stored outside the repository checkout. Removing or recreating the application container does not remove the named storage volume unless the volume itself is explicitly deleted.

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

Current integration coverage includes user and project persistence, session lifecycle behavior, password credential persistence, versioning transactions and constraints, content-object and project-file persistence, cross-project content reuse, storage metadata conflicts, conversion-job creation and lifecycle transitions, concurrent-safe pending-job claiming, startup recovery, derived conversion-output persistence, successful reconversion selection, output-content reuse and conflicts, transaction rollback, and atomic multi-row persistence paths.

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
milestone/conversion-jobs
milestone/glb-preview-pipeline
```

Within each milestone, work remains divided into small logical commits. Completed milestones are merged into `main` through pull requests.

The goal is to keep both the codebase and Git history understandable as the project grows.

---

## Engineering Principles

* **Original engineering data is authoritative.** Native CAD files are the source of truth; generated previews are derived artifacts.
* **History should be immutable.** Historical revisions should not be rewritten.
* **Persistence should be atomic where correctness requires it.** Related database state is committed together or rolled back together.
* **Immutable content should not be destructively compensated.** A failed metadata write must not delete a content-addressed object that may be shared by another reference or concurrent upload.
* **Identity comes from authentication.** Clients should not declare ownership of authenticated resources.
* **Infrastructure stays simple until complexity is justified.** PostgreSQL and a modular monolith are preferred over premature distributed services.
* **Durable background work belongs in durable state.** Conversion jobs are persisted before execution and recovered explicitly rather than relying on in-memory queues.
* **Derived artifacts remain replaceable.** Conversion and preview outputs must be reproducible from authoritative source files and should not become the source of truth.
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
* [x] authenticated branch rename API
* [x] authenticated branch deletion API
* [ ] broader branch management
* [x] branch-head DAG traversal and history API
* [ ] immutable-history hardening and cycle prevention
* [ ] merge and conflict-resolution workflow
* [x] content-addressed storage
* [x] project-file metadata and content references
* [x] exact revision-file snapshot persistence
* [x] authenticated revision-file snapshot creation and listing API
* [x] authenticated source-file upload
* [x] authenticated project-file metadata listing and detail API
* [x] physical file download/open API
* [x] conversion jobs
* [x] GLB preview pipeline
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

As a portfolio project, 3Default demonstrates practical engineering across Go, PostgreSQL, SQL and schema design, REST APIs, authentication, Docker, generated code workflows, integration testing, content-addressed storage, streaming file uploads and downloads, durable background-job orchestration, concurrency-safe PostgreSQL work claiming, transactional derived-output persistence, authenticated GLB preview streaming, Git, and product-oriented architecture.

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
