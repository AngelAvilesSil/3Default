# 3Default

3Default is a version-control and visual-collaboration platform for 3D projects, currently focused on mechanical CAD and engineering design workflows.

The goal is to make engineering design history easier to understand through explicit revisions, parallel branches, intentional merges, and browser-based 3D review.

> **The original CAD file is the source of truth. Derived previews and converted formats are reproducible artifacts, not authoritative engineering data.**

This repository is a ground-up reconstruction of an earlier 3Default MVP. It is being developed first as a strong software-engineering portfolio project while keeping the architecture practical enough to evolve into a real product without requiring a major rewrite.

---

## Project Status

**Current phase: unified public/authenticated navigation, building on the read-only project overview, branch summaries, My Projects, project creation, and the existing backend and 3D-preview foundations**

Implemented foundations include:

* Docker-based development and VS Code Dev Container support
* Go backend and Vue 3 + TypeScript frontend
* PostgreSQL persistence with Tern migrations and sqlc
* OpenAPI-first HTTP contracts
* health and readiness endpoints
* project persistence and authenticated project creation, listing, detail reads, and metadata updates
* owner-scoped, case-insensitive project-name uniqueness enforced by PostgreSQL, with HTTP `409 Conflict` for duplicate creation or renaming
* typed frontend project listing, creation, detail, and branch-listing API clients
* authenticated My Projects browser listing with loading, empty, retryable-error, and expired-session handling
* linked, protected project overview at `/projects/:projectId`, with real project metadata, loading, unavailable, retryable-error, and expired-session states
* read-only project branch summaries with nullable head-revision information, independent loading and retry, project-ID consistency checks, and cancellation of obsolete requests
* protected Create Project form with required-name validation, optional description, duplicate-name feedback, cancellation, and return to My Projects
* frontend project creation and listing integration tests, plus real-browser verification of persistence and duplicate-name handling
* project overview navigation and lifecycle tests, branch-summary tests, and real-browser verification of project details, branch states, direct links, responsive overflow, and keyboard navigation
* project revision and branch persistence with automatic `main` branch creation
* atomic revision creation with optimistic branch-head updates
* authenticated branch creation, rename, and deletion, branch listing and history traversal, revision reads, and revision creation
* immutable SHA-256 content-addressed filesystem storage with physical deduplication
* PostgreSQL project-file metadata and exact revision-file snapshot persistence
* database-enforced immutability of project-file metadata and content references
* transactionally finalized revision snapshots with immutable file membership
* database protection of revision metadata, ancestry, and historical deletion
* authenticated multipart source-file upload with project ownership checked before physical storage
* authenticated project-file metadata listing and detail reads
* authenticated streaming source-file download with integrity verification
* authenticated exact revision-file snapshot creation and listing
* durable PostgreSQL-backed CAD conversion jobs with `pending`, `running`, `succeeded`, and `failed` states
* authenticated conversion-job creation, project-file job history, and project-scoped job detail APIs
* embedded conversion worker started by the Go application, with PostgreSQL-backed claiming, at-least-once execution semantics, graceful requeue, and startup recovery
* five-minute per-job execution deadline with persistent timeout failures and graceful-shutdown requeue
* immutable derived-preview persistence using the shared content-addressed storage layer and transactional conversion-output metadata
* GLB 2.0 pass-through converter with structural validation
* format routing for `.glb`, `.step`, and `.stp` source files
* MayoConv 0.10.0 STEP-to-GLB adapter with temporary-file cleanup and output validation
* Linux Mayo subprocess-group termination on cancellation, with bounded output-pipe waiting
* Docker-packaged Mayo executable with optional `MAYO_EXECUTABLE` application configuration
* authenticated latest-successful GLB preview streaming through `/api/projects/{projectId}/files/{projectFileId}/preview`
* routed Three.js browser preview through `/projects/{projectId}/files/{projectFileId}/preview`
* same-origin authenticated preview loading with explicit loading, unavailable, request-error, and render-error states
* GLTFLoader-based GLB parsing with OrbitControls, automatic camera fitting, resize handling, animation-loop lifecycle management, and deterministic model-resource disposal
* abortable preview requests with stale-response protection when the selected project or file changes
* lazy-loaded preview route so Three.js stays out of the initial application bundle
* persistent Docker storage configured through `STORAGE_ROOT`
* atomic user registration and password-credential creation
* password creation policy and local weak-password screening
* Argon2id password hashing
* PostgreSQL-backed server-side sessions
* secure opaque session tokens and SHA-256 token hashing
* secure host-only session cookies
* login, logout, and authenticated-user HTTP flows
* typed frontend authentication API client using same-origin session cookies
* Pinia authentication state with session restoration, stale-response protection, and serialized login/logout operations
* shared session-aware site header with a 3Default home link, My Projects navigation, authenticated user identity, and Sign out
* shared landing page at `/` for visitors and authenticated users, with nonblocking session restoration and neutral navigation while verification is pending
* centralized frontend styling tokens, shared form-field styles, and reusable button, text-field, and textarea components
* Vue login form with credential-error, rate-limit, request-error, and loading feedback
* public landing and login routes, protected My Projects, Create Project, Project Overview, and preview routes, recoverable session-verification errors, and a legacy `/app` redirect to `/`
* safe post-login return navigation that preserves protected paths and query parameters, falling back to `/`
* real-browser verification of registration, login, session persistence, logout, deep links, and temporary API-outage recovery
* unsafe cross-origin browser request protection
* unit and PostgreSQL integration tests

The authentication, core project-versioning, file-storage, durable conversion-job, and first GLB preview-pipeline foundations are now implemented. The backend supports branch listing, creation, rename, and deletion, project-scoped revision reads, atomic revision creation with optimistic branch-head concurrency, branch-head history traversal, immutable content-addressed source-file storage, owner-scoped project-file upload, metadata reads, streaming source-file download, exact revision-file snapshots supplied during revision creation, authenticated revision-file snapshot reads, durable conversion-job creation and inspection, persisted derived preview outputs, and an embedded worker that is started by `main` and finalizes successful jobs only together with their output metadata. With Mayo configured, the conversion worker supports STEP/STP-to-GLB conversion alongside structurally validated GLB 2.0 pass-through. Without Mayo, the worker preserves the original GLB pass-through behavior. The routed Vue application now consumes authenticated GLB previews through a Three.js browser viewer. PostgreSQL now protects project-file metadata, finalized revision-file membership, and committed revision metadata and ancestry against direct historical rewrites. Comprehensive revision-graph cycle validation, additional native CAD conversion formats, revision-aware visual comparison, and user-facing merge/conflict-resolution workflows remain future work.

**Current browser UI scope:** The shared landing page at `/` serves both visitors and authenticated users. A session-aware header provides My Projects navigation, authenticated user identity, and Sign out. The browser supports login, protected navigation, an authenticated My Projects listing, project creation, a read-only project overview at `/projects/:projectId`, and direct authenticated GLB preview routes. Project names in My Projects link to the overview, which displays owner-scoped project metadata and actual branches with their current head-revision identifiers or an explicit empty-head state. Project creation returns to a refreshed listing. Browser registration, project metadata editing, integrated project-file browsing and upload, interactive branch switching, historical revision navigation and comparison, merge/conflict-resolution workflows, and public project discovery remain future work. The existing Three.js preview is a separate direct route and is not yet integrated into the project overview. The backend project, file, and versioning APIs provide capabilities beyond the current browser UI.

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

During development, Vite proxies `/api/*` requests to the Go service. The typed frontend project client uses this same-origin path to list and create projects, retrieve individual project details, and list project branches. The existing Vue Router lifecycle reloads My Projects after successful creation without a separate project state store. The protected project overview first retrieves project metadata, then independently loads its branches. Project and branch requests use abort controllers and stale-response protection; branch failures can be retried without discarding successfully loaded project metadata. Backend ownership checks remain authoritative, and the overview rejects branch records associated with another project. The separate project-file preview route remains lazy-loaded so the Three.js viewer and its rendering dependencies are fetched only when browser visualization is requested.

Frontend authentication is divided between a typed same-origin HTTP client, a Pinia session store, Vue Router navigation guards, and small reusable view components. Protected routes await session restoration before deciding access. The shared application shell also starts a nonblocking session check after mounting when the session state is unknown, allowing the public landing page to render regardless of API availability. The header remains neutral during verification, and a verification failure leaves public content accessible with manual Sign in available. The browser stores authenticated user state in memory; the opaque session credential remains in the server-managed, HttpOnly cookie. Backend session resolution and project ownership checks remain authoritative for data access.

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
* Three.js
* Vitest
* Prettier
* Oxlint
* ESLint

### Development

* Docker Desktop
* Docker Compose
* VS Code Dev Containers
* Node.js 24 frontend container
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

### Frontend Session and Navigation

The frontend authentication client (`web/src/api/auth.ts`) calls the registration-independent login, current-user, and logout endpoints using same-origin browser credentials. The client translates expected authentication HTTP responses into typed results and errors rather than exposing session tokens to JavaScript.

The Pinia session store (`web/src/stores/session.ts`) maintains the current authenticated user and a lifecycle state of `unknown`, `checking`, `authenticated`, `unauthenticated`, or `error`. It restores existing server sessions, distinguishes confirmed unauthenticated responses from network or server verification failures, prevents stale restoration responses from overwriting newer state, and serializes login and logout operations.

Vue Router provides the following browser routes:

| Route | Access | Current purpose |
| --- | --- | --- |
| `/` | Public | Shared landing page for visitors and authenticated users, with session-aware navigation |
| `/login` | Public | Login form; authenticated users return to a valid protected destination or `/` |
| `/app` | Public compatibility redirect | Client-side Vue Router redirect to `/`, for signed-in and signed-out visitors |
| `/projects` | Authenticated | My Projects listing with empty, loading, and error/retry states |
| `/projects/new` | Authenticated | Create Project form with name validation and optional description |
| `/projects/:projectId` | Authenticated | Read-only project overview with project metadata and actual branch/head-revision summaries |
| `/session-unavailable` | Public | Recoverable session-verification error and retry action |
| `/projects/:projectId/files/:projectFileId/preview` | Authenticated | Existing lazy-loaded Three.js preview |

For protected navigation, the router waits for session restoration before deciding access. A confirmed unauthenticated result redirects to `/login` while preserving the requested protected URL. A session-verification failure instead opens `/session-unavailable`, where the user can retry without assuming that their session has been revoked.

After successful login, the application returns to the requested destination only when it is a validated, existing protected route, preserving its supported path and query parameters. Ordinary login without a return destination, and unrecognized, public, or external return destinations, lead to `/`. An already-authenticated visitor opening `/login` follows the same safe return-path rule. The login form itself remains independent of route-navigation decisions.

The UI foundation includes centralized CSS tokens, reusable `AppButton`, `AppTextField`, and `AppTextArea` components, and shared text-control styling in `web/src/styles/form-fields.css`. `LoginForm` handles invalid credentials, request throttling, general failures, and in-progress submissions. A successful login updates Pinia session state and navigates through the login view. The shared `SiteHeader` displays My Projects and the authenticated user's display name, and provides the common Sign out action across application pages. Successful logout revokes the server session, clears authenticated browser state, and returns to `/`. A failed logout retains the existing authenticated state and displays retryable feedback.

Registration remains available through `POST /api/auth/register`, but a browser registration form has not yet been implemented. The shared landing page remains public, `/projects` provides the authenticated project listing, and `/projects/new` provides project creation. The read-only project overview is now implemented, but integrated file browsing, interactive branch and revision navigation, and browser metadata editing remain future work.

Frontend navigation guards are a user-experience boundary, not a substitute for API authorization. The Go backend continues to enforce session validity and owner-scoped access to project resources.

---

## Projects and API

Projects are private by default and currently include an ID, owner user ID, name, optional description, visibility, and timestamps.

For authenticated project operations, ownership is derived from the server-side session rather than accepted from the request.

Project names must be nonblank and are unique within each owner's projects, ignoring letter case. PostgreSQL enforces this through the `projects_owner_name_case_insensitive_unique` index on `(owner_user_id, lower(name))`. The application maps a duplicate name during `POST /api/projects` or `PATCH /api/projects/{projectId}` to HTTP `409 Conflict`. Different owners may use the same project name. Creating a project also creates its initially empty `main` branch in the same database transaction.

The Vue creation form trims the submitted name and optional description. An empty name is rejected before the HTTP request, while a duplicate name is reported beside the name field without discarding the entered values. A successful HTTP `201 Created` returns the browser to My Projects, which fetches the current listing again.

The My Projects listing now links each project name to `/projects/:projectId`. This protected route retrieves project metadata through `GET /api/projects/{projectId}` and displays the name and optional description. It provides loading, unavailable, request-error, retry, and expired-session handling while preserving the requested project URL when login is required.

After project metadata loads, the overview independently requests `GET /api/projects/{projectId}/branches`. The browser displays the branches returned by the backend in their response order. A `null` `headRevisionId` is shown as **No revisions yet**; a populated head is displayed as a revision identifier without inventing revision metadata. Empty branch collections are supported, and branch errors can be retried without reloading project metadata. Changing projects or leaving the view cancels obsolete requests, and returned branch project IDs are checked before display. This is informational branch browsing only: there is no interactive branch selection or historical file-snapshot view yet.

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
GET  /api/projects/{projectId}/files/{projectFileId}/preview
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

Revision creation, exact revision-file references, target-branch advancement, and revision finalization happen atomically in one database transaction. A newly inserted revision starts with `membership_finalized = false`. The service inserts the complete file membership, advances the target branch head if its expected value still matches, sets `membership_finalized = true`, and commits. A deferred database constraint prevents an unfinalized revision from remaining committed. If any required operation fails, the revision and its membership are rolled back.

The client must provide `expectedHeadRevisionId` as an optimistic-concurrency precondition:

* `null` means the caller explicitly observed an empty branch.
* a UUID means the caller observed that revision as the branch head.
* omitting the field is invalid.

Revision creation also requires `projectFileIds`, which represents the complete project-file snapshot for the new revision:

* `[]` means the caller explicitly creates an empty file snapshot.
* a list of UUIDs means exactly those project files belong to the revision snapshot.
* omitting `projectFileIds` or supplying `null` is invalid at the HTTP API.
* duplicate IDs and the nil UUID are invalid, and every referenced file must exist in the same project.

The snapshot is not a delta from the primary parent and no files are inherited implicitly. There are no mutable attach or detach endpoints. PostgreSQL also prevents revision-file membership updates and independent deletions, rejects insertion into finalized revisions, and prevents truncation of the membership table. Once finalized, a revision cannot be reopened to change its snapshot.

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

### Revision Integrity Guarantees

Historical revisions are append-only at both the application and database boundaries. The service and HTTP API expose no revision update or delete operations. PostgreSQL additionally rejects direct updates to stored revision identity, project association, author, message, creation timestamp, and first-parent or merge-parent references. The only permitted revision-row update is the initial transition from unfinalized to finalized without changing historical fields.

Independent revision deletion and revision-table truncation are rejected. Deleting the owning project remains supported and cascades through its revision history. Existing constraints also require same-project parent references, prohibit self-parenting and duplicate parents, and require a primary parent when a merge parent is present.

These safeguards protect committed history against subsequent rewrites, but they are not a separate comprehensive cycle validator for every possible graph created through direct SQL insertion. Explicit graph-cycle validation remains future work.

Other future work includes broader branch management, additional CAD-format support, revision-aware visual comparison, and user-facing merge/conflict-resolution workflows.

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

The revision-file relation is part of revision creation. The revision row, its exact project-file references, the target branch-head update, and revision finalization are persisted in one transaction. A missing or wrong-project file, a duplicate file ID, or a branch-head conflict rolls back the candidate revision and its file references.

PostgreSQL rejects direct `UPDATE` operations on `project_files`, protecting the stored source-file metadata and content hash against rewriting. Project-file deletion remains subject to existing foreign-key and project-cleanup rules. Revision-file membership is frozen after finalization, and database triggers prevent standalone membership updates and deletions. Legitimate project deletion can still cascade through the associated records.

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

The current HTTP API exposes project-file metadata, exact revision-file snapshot metadata, authoritative physical source-file download, durable conversion-job orchestration, and retrieval of the latest successful GLB preview for a project file. Derived preview bytes reuse the immutable content-addressed storage layer, while their conversion-job relationship is stored separately so source `project_files` remain authoritative. HTTP range requests, conditional caching/ETags, and conversion support for additional native CAD formats remain future work. Browser visualization is now implemented through the routed Three.js preview viewer.

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
   ├── execution timeout ───► failed
   ├── graceful shutdown ───► pending
   └── process crash ───────► recovered to pending on startup
```

The schema enforces the state/timestamp relationships, keeps `attempt_count`, stores a trimmed `last_error` only for failed jobs, and retains completed jobs as history. At most one `pending` or `running` job may exist for the same project file at a time. After a job reaches `succeeded` or `failed`, a later conversion request may create another job for that file.

The embedded worker claims the oldest pending job with PostgreSQL row locking and `FOR UPDATE SKIP LOCKED`, increments its attempt count, and transitions it to `running`. Execution is intentionally **at least once** rather than exactly once. A converter must therefore tolerate the possibility that a previously started job is executed again after process failure and startup recovery.

Graceful cancellation requeues the currently running job instead of marking it failed. Startup recovery also requeues rows left in `running` by an earlier process. This recovery model is correct for the current single-process, single-worker deployment shape. A future multi-process or distributed worker topology would require a lease or heartbeat mechanism rather than globally requeueing all running jobs.

After claiming a job, the worker applies a fixed five-minute execution deadline to source retrieval, conversion, and derived-output storage. If the deadline expires, the job is marked `failed` with a persisted timeout reason. Application shutdown instead takes precedence and requeues interrupted work. Database claiming and finalization retain their existing separate execution contexts.

On Linux, the Mayo command runner starts the converter in a separate process group and sends `SIGKILL` to that group when the command context is canceled. A two-second `WaitDelay` bounds waiting for inherited output pipes. Other operating systems retain Go's direct-process cancellation with the same pipe-wait bound. These mechanisms do not provide CPU or memory quotas, full process isolation, or guaranteed termination of processes that deliberately detach into other process groups.

The converter itself is behind a replaceable adapter boundary. When `MAYO_EXECUTABLE` is configured, the format router delegates `.glb` files to the structurally validating GLB pass-through adapter and `.step`/`.stp` files to MayoConv 0.10.0. The Mayo adapter stages STEP input in temporary files, runs the external converter, validates the resulting GLB, and cleans up temporary files. The resulting preview uses media type `model/gltf-binary`. Unsupported formats fail rather than being mislabeled as converted previews. When Mayo is unconfigured, the worker falls back to the previous content-based GLB pass-through behavior.

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

### Conversion Execution Hardening Verification

Worker regression tests verify that conversion and output-storage deadlines record failed jobs, while application shutdown continues to requeue interrupted work. Linux command-runner tests launch a real shell and child process to verify process-group termination on cancellation. Process regressions passed ten consecutive runs, alongside the backend test suite, race detector, static analysis, and Windows cross-compilation.

These tests verify the worker timeout and subprocess-cancellation mechanisms. They do not establish real-Mayo timeout behavior under every workload or enforce operating-system resource quotas.

### STEP-to-GLB Conversion Verification

The STEP conversion pipeline was validated with a disposable Docker Compose project, a fresh PostgreSQL database, and an OpenCascade `screw.step` fixture.

The end-to-end test registered and authenticated a temporary user, created a project, uploaded the original STEP source, verified the downloaded source bytes were unchanged, created a durable conversion job, and waited for the embedded worker to finish. MayoConv 0.10.0 produced a 24,112-byte GLB containing one mesh. The persisted job reached `succeeded` on its first attempt, the authenticated preview endpoint streamed valid GLB 2.0 content, and an anonymous preview request was rejected.

The generated model was also opened in the Vue/Three.js browser viewer, where rendering, rotation, panning, and zooming were verified manually. The disposable database, volumes, and test scripts were removed after verification.

This test confirms the supported STEP path, not arbitrary native CAD compatibility or production-scale conversion performance.

---

### GLB Preview Retrieval

The latest successful preview for a project file is exposed through:

```text
GET /api/projects/{projectId}/files/{projectFileId}/preview
```

The endpoint is authenticated and owner-scoped. The service first verifies project ownership and that the project file belongs to that project, then resolves the newest persisted output attached to a `succeeded` conversion job. Earlier successful outputs remain retained as conversion history; the endpoint selects the latest successful one rather than mutating or replacing historical rows.

Only persisted outputs with media type `model/gltf-binary` are served by this endpoint. A project or project file that is missing or not owned by the current user returns `404`, and a project file with no successful preview also returns `404`. Missing, unreadable, or integrity-invalid physical content and incompatible persisted preview media types are treated as internal inconsistencies and return `500`.

Successful responses stream the GLB body directly as `model/gltf-binary`. The response intentionally does not use `Content-Disposition: attachment`, because the resource is intended for browser/Three.js consumption rather than forced download. The generated OpenAPI transport closes the returned content stream after copying it to the response.

The conversion service, preview service, and embedded worker are wired into the HTTP application in `main`. `NewConfiguredConverter` selects GLB pass-through alone when Mayo is absent, or GLB/STEP format routing when Mayo is configured. Successful jobs still use the transactional output-finalization path, and the converter boundary remains replaceable for additional CAD formats.

---

## Three.js Browser Preview

The Vue application exposes a direct browser route for a project-file preview:

```text
/projects/{projectId}/files/{projectFileId}/preview
```

Vue Router lazy-loads the preview feature only when this route is visited. The `ProjectFilePreview` coordinator starts an abortable same-origin request to the authenticated backend preview endpoint, maps `404` to a normal unavailable state, maps other request failures to an error state, and passes a successful GLB `ArrayBuffer` to the rendering component. Changing either route identifier aborts the previous request and prevents a stale response from replacing the newly selected file.

`ThreePreviewCanvas` owns the WebGL-specific lifecycle. It parses binary GLB data with `GLTFLoader`, renders it with Three.js, provides orbit/pan/zoom interaction through `OrbitControls`, and automatically fits the camera to the loaded model. The component also observes container resizing, caps device pixel ratio, runs damped controls through the renderer animation loop, rejects stale asynchronous model loads, and releases model geometry, materials, textures, closeable texture sources, controls, the animation loop, and the renderer when models are replaced or the component unmounts.

The route currently depends on an existing authenticated session and a successful persisted GLB conversion output. With Mayo configured, STEP and STP files can produce browser-viewable GLB previews after their conversion jobs succeed. Uploading a source file and requesting a conversion job remain separate API operations; the preview route does not start conversion jobs. Other native CAD formats remain unsupported.

The frontend router now also guards this preview route. An unauthenticated visitor is sent to the login screen and can return to the originally requested preview URL after authentication. This routing behavior does not itself prove that a particular file has a valid conversion output; a successful preview still requires an existing, accessible project file and persisted GLB content.

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

Node and npm intentionally run in the dedicated `web` Compose service rather than being installed in the Go development container. The Dev Container mounts the same `web-node-modules` named volume at `/workspace/web/node_modules`, which lets VS Code and the workspace TypeScript server resolve the exact frontend dependencies installed by the `web` service without duplicating the Node toolchain inside the Go container.

The application requires both `DATABASE_URL` and `STORAGE_ROOT`. Docker Compose and the Dev Container configure these automatically for local development.

STEP conversion additionally requires `MAYO_EXECUTABLE`. The API image in `docker/api.Dockerfile` downloads a pinned MayoConv 0.10.0 AppImage, verifies its SHA-256 checksum, extracts it into `/opt/mayo`, and sets `MAYO_EXECUTABLE=/opt/mayo/AppRun`. Mayo runs directly without Xvfb. When this variable is unset or blank, such as when running Go directly in a Dev Container without Mayo installed, the worker retains GLB pass-through but cannot convert STEP files.

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
docker compose up -d --build db api web
```

Development services:

* Go API: `http://localhost:8080`
* Vue/Vite: `http://localhost:5173`
* PostgreSQL data: persistent `postgres-data` Docker volume
* immutable source and derived preview content: persistent `storage-data` Docker volume mounted at `/var/lib/3default/storage`

For browser development, open [http://localhost:5173](http://localhost:5173).

The current browser entry points are:

* `http://localhost:5173/` for the shared landing page, available to visitors and authenticated users.
* `http://localhost:5173/login` for signing in.
* `http://localhost:5173/app` as a compatibility URL that redirects to `/`.
* `http://localhost:5173/projects` for the authenticated My Projects listing.
* `http://localhost:5173/projects/new` for creating a project.
* `http://localhost:5173/projects/<projectId>` for an authenticated, read-only project overview and its current branches.

The browser currently provides login and logout but not a registration form. On a new local database, create a development account through `POST /api/auth/register` using the API contract's `email`, `displayName`, and `password` fields, then sign in through `/login`. Registration does not automatically establish a session. Do not commit development credentials or session-cookie values to the repository.

Ordinary sign-in returns to `/`, where the shared header provides a My Projects link. Open `/projects` to see your projects or navigate to `/projects/new` to create one. A new project starts private and has an empty `main` branch. The creation form requires a project name and accepts an optional description. After a successful submission, the browser returns to My Projects. Duplicate names within the same account, including differently capitalized names, are rejected.

From My Projects, select a project name to open its overview. A newly created project's `main` branch initially has no head revision and appears as **No revisions yet**. Existing branch heads display their revision identifiers. The overview reads actual backend data and does not yet offer project-file browsing, branch switching, or revision comparison. The separate authenticated GLB preview route continues to work independently.

In this Docker development setup, Vite serves the Vue source, while the API service launches the Go application with `go run`. When Go source changes, restart the `api` service so the running process compiles and loads those changes. If the browser unexpectedly continues showing an older Vue template, restarting `web` can refresh the development server. These restarts do not delete the named PostgreSQL or storage volumes.

A source upload does not automatically enqueue conversion. After uploading a `.step` or `.stp` file, create a conversion job through the authenticated conversion-job POST endpoint and wait for its status to become `succeeded`. The generated GLB can then be retrieved through the authenticated preview endpoint.

An authenticated project-file preview can be opened directly at:

```text
http://localhost:5173/projects/<projectId>/files/<projectFileId>/preview
```

The page requires the normal server-side login session. Its browser request is sent through the same-origin `/api/*` path, which Vite proxies to the Go API service during development.

Uploaded source content and generated preview content are intentionally stored outside the repository checkout. Removing or recreating the application container does not remove the named storage volume unless the volume itself is explicitly deleted.

---

## Database and Code Generation

Database migrations live in `db/migrations/`.

Revision-integrity enforcement is implemented through three migrations:

* `00008_project_file_immutability.sql` rejects updates to existing project-file records.
* `00009_revision_membership_finalization.sql` introduces commit-time revision finalization and protects revision-file membership.
* `00010_revision_history_immutability.sql` protects historical revision metadata, ancestry, and deletion behavior.

Migration 00009 marks pre-existing revisions as finalized. Newly inserted revisions must be finalized before their creation transaction commits.

Migration `00011_project_name_uniqueness.sql` adds owner-scoped, case-insensitive project-name uniqueness. The project store translates violations of its specific unique index into application-level name conflicts rather than treating them as generic persistence failures.

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

Use the explicit `./cmd/... ./internal/...` package scopes from the Dev Container rather than `go test ./...`. The Dev Container now mounts the frontend `node_modules` volume for editor tooling, and third-party npm packages may themselves contain unrelated Go source trees.

Run the frontend unit suite, TypeScript check, and production build from the host through the dedicated `web` service:

```powershell
docker compose exec -T web npm run test:unit -- --run
docker compose exec -T web npm run type-check
docker compose exec -T web npm run build
```

The preview route is lazy-loaded. A production build therefore keeps the initial application bundle separate from the larger on-demand Three.js preview chunk.

Frontend Vitest coverage includes typed authentication and project APIs (including project detail and branch listing), Pinia session lifecycle, reusable UI primitives, login-form behavior, My Projects loading/error/retry behavior, Create Project validation and error handling, protected navigation, safe return-path handling, login-view behavior, shared-header logout integration, nonblocking public-session restoration, legacy `/app` compatibility, and session-verification retry behavior. Project overview tests verify metadata loading, unavailable projects, expired sessions, retry, and stale-request cancellation. Separate branch-summary tests cover empty and populated heads, multiple branches, loading, independent retry, inconsistent project IDs, unauthorized responses, and switching projects while branch requests remain pending. A real-router integration test verifies navigation from an empty listing through project creation and back to a refreshed listing. Existing GLB preview and Three.js component tests remain part of the same suite.

At the Unified Navigation milestone checkpoint, the complete frontend suite passed **219 tests across 23 test files**, with successful TypeScript checking and production build. The shared navigation has dedicated coverage for authenticated, unauthenticated, pending, and failed-verification states; successful and failed logout; and legacy `/app` redirects.

For non-mutating frontend quality checks, use `npx prettier --check src`, `npx oxlint .`, and `npx eslint . --no-cache` inside the `web` service. The repository's `npm run lint` scripts use automatic fixes; do not run them during a read-only review. Repository-wide lint has pre-existing findings outside this milestone. Passing tests, a production build, and focused changed-file lint checks must not be described as a clean repository-wide lint result.

For manual local-browser authentication acceptance:

1. Start `db`, `api`, and `web`, then open a fresh browser session at `http://localhost:5173/`.
2. Verify that `/` renders for visitors, `/login` provides the login form, and a direct visit to `/app` redirects to `/`.
3. Register a disposable development user through the API, then sign in through the browser. Confirm ordinary login returns to `/` and the header shows My Projects, the user's display name, and Sign out.
4. Verify the `__Host-3default_session` cookie's `Secure` and `HttpOnly` attributes without copying its value.
5. Refresh `/` and verify that the authenticated header returns after session restoration without a misleading Sign in flash.
6. Verify that the shared landing page remains accessible while authenticated and that My Projects is reachable through the header.
7. Sign out and confirm that the browser returns to `/` with Sign in visible. Verify `/app` still redirects to `/`, while direct navigation to the protected `/projects` route requires login.
8. From a signed-out session, request a protected preview deep link and verify that login returns to the requested path with its query string intact.
9. For local failure-recovery verification, stop only the API service, refresh a protected page, confirm the recoverable session-error screen, restart the API, and use **Try again** to restore the existing session.

A fabricated preview URL is sufficient to check post-login navigation, but it does not constitute successful CAD rendering verification. Actual preview rendering requires an existing project file and successful conversion output.

These normal-session, deep-link, and temporary API-outage scenarios passed manual browser acceptance during the frontend-authentication milestone.

During the Unified Navigation milestone, **11 functional browser checks** passed, covering public and authenticated landing behavior, session restoration, My Projects and project-overview navigation, the 3Default home link, legacy `/app` redirects, logout, protected-route login, and ordinary login. **Five additional responsive and accessibility checks** passed at approximately 394 px mobile, 768 px tablet, and desktop widths, including keyboard navigation and browser-console verification. No unexpected console errors or horizontal-overflow issues were reported.

For manual My Projects and Create Project acceptance:

1. Sign in and open `/projects`; verify the list or empty state and its Create Project navigation.
2. Open `/projects/new` and confirm that an empty name produces inline validation without creating a project.
3. Create a private project with a unique name and optional description, then verify that navigation returns to My Projects.
4. Refresh the listing and confirm that the new project and its metadata persist.
5. Attempt a duplicate name with different capitalization; verify HTTP `409 Conflict` and the field-specific message.
6. Edit the rejected name and confirm that the stale duplicate-name error disappears without resubmission.
7. Cancel and verify that the browser returns to My Projects without creating another project.
8. Check the form, shared text controls, and navigation at normal and narrow browser widths.

These project creation, persistence, duplicate-name, recovery, cancellation, and visual checks passed real-browser acceptance during the My Projects milestone. The frontend suite passed 161 tests across 18 test files at that earlier milestone checkpoint.

For manual Project Workspace acceptance:

1. Sign in and open `/projects`; confirm that existing project names link to their protected overview URLs.
2. Open a project and verify its real name, optional description, and branch information.
3. Confirm that the default `main` branch shows **No revisions yet** when its head is null, without inventing historical revision data.
4. In the browser Network panel, verify successful project-detail and branch-listing API requests.
5. Refresh the project URL directly, navigate back to My Projects, and verify that the correct project remains accessible.
6. Check the browser console, keyboard focus and activation of navigation links, and layout at narrow and tablet widths.

These Project Workspace checks passed real-browser acceptance. The tested project displayed its persisted metadata and empty `main` branch correctly; the project and branch API requests returned HTTP `200`; direct refresh and back navigation worked; no unexpected console errors were observed; keyboard navigation worked; and document-level horizontal overflow was absent at approximately 394 px and 768 px. The full frontend suite passed **213 tests across 22 test files** at this milestone checkpoint. This acceptance covers the read-only overview, not integrated CAD file browsing, historical revision comparison, or merge workflows.

Run PostgreSQL integration tests:

```bash
go test -count=1 -tags=integration -v ./internal/database
```

Current integration coverage includes user and project persistence, session lifecycle behavior, password credential persistence, versioning transactions and constraints, content-object and project-file persistence, cross-project content reuse, storage metadata conflicts, conversion-job creation and lifecycle transitions, concurrent-safe pending-job claiming, startup recovery, derived conversion-output persistence, successful reconversion selection, output-content reuse and conflicts, transaction rollback, and atomic multi-row persistence paths.

Revision-integrity regressions additionally cover project-file update rejection, commit-time revision finalization, immutable revision-file membership, concurrent insertion and finalization races, metadata and ancestry rewrite rejection, independent revision deletion, truncation protection, and project-deletion cascades. The integrity migrations were also verified through rollback and reapplication against the development database.

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
milestone/threejs-preview-viewer
milestone/revision-integrity-hardening
milestone/frontend-authentication
milestone/frontend-my-projects
milestone/frontend-project-workspace
milestone/frontend-unified-navigation
```

Within each milestone, work remains divided into small logical commits. Implementation commits are followed by a separate README/documentation audit and documentation commit when updates are needed. Completed milestones are merged into `main` through pull requests using **Rebase and merge**, preserving linear history.

For this Windows 11 development workflow, use **Windows Git Bash** for Git staging, diff review, commits, branch operations, and pushes; the **Go Dev Container** for source changes and Go tooling; and **Windows PowerShell** for Docker Compose commands and frontend tooling in the `web` service.

The goal is to keep both the codebase and Git history understandable as the project grows.

---

## Engineering Principles

* **Original engineering data is authoritative.** Native CAD files are the source of truth; generated previews are derived artifacts.
* **History should be immutable at the persistence boundary.** Project-file metadata, finalized revision membership, and revision metadata and ancestry must not be silently rewritten.
* **Persistence should be atomic where correctness requires it.** Related database state is committed together or rolled back together.
* **Immutable content should not be destructively compensated.** A failed metadata write must not delete a content-addressed object that may be shared by another reference or concurrent upload.
* **Identity comes from authentication.** Clients should not declare ownership of authenticated resources.
* **Infrastructure stays simple until complexity is justified.** PostgreSQL and a modular monolith are preferred over premature distributed services.
* **Durable background work belongs in durable state.** Conversion jobs are persisted before execution and recovered explicitly rather than relying on in-memory queues.
* **Derived artifacts remain replaceable.** Conversion and preview outputs must be reproducible from authoritative source files and should not become the source of truth.
* **Frontend session state is not the session credential.** The browser stores user-facing authentication state in Pinia while the opaque credential stays in an HttpOnly, server-managed cookie.
* **Navigation is not authorization.** Protected routes provide correct browser flow, while backend session validation and owner-scoped API checks enforce access to engineering data.
* **Branch pointers are not immutable revisions.** A branch identifies its current head; historical file membership belongs to exact revisions. The browser must not imply file-level history or changes that its data cannot establish.
* **Project and branch loading remain independently recoverable.** An unavailable branch list should not erase valid project metadata, and obsolete requests must not overwrite a newly selected project's state.
* **Browser responsibilities stay separated.** HTTP loading and UI state belong to the preview coordinator, while WebGL parsing, camera control, rendering, and GPU-resource cleanup belong to the Three.js canvas component.
* **Heavy browser features load only when needed.** The preview route is lazy-loaded so Three.js does not inflate the initial application bundle.
* **Development should resemble production.** The development architecture intentionally follows the expected production shape.
* **New substantial UI work requires a design checkpoint.** Review the intended user path and visual references before implementation, then prefer reusable primitives, semantic design tokens, and maintainable Vue components over scattered styling or premature UI frameworks.

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
* [x] typed frontend authentication API client
* [x] Pinia session restoration and lifecycle management
* [x] reusable UI tokens, button, and text-field components
* [x] browser login form with loading and error states
* [x] public landing and protected route foundation
* [x] shared landing page with nonblocking session restoration
* [x] shared session-aware header with My Projects navigation and logout
* [x] legacy `/app` compatibility redirect to `/`
* [x] real-browser unified navigation, responsive, and keyboard acceptance
* [x] safe return navigation to protected pages
* [x] recoverable session-verification failure handling
* [x] browser acceptance of login, refresh, logout, and session recovery
* [ ] user-facing registration and account recovery workflows

### Projects

* [x] project persistence
* [x] project application service
* [x] authenticated project creation API
* [x] project listing and detail APIs
* [x] metadata update API
* [x] owner-scoped, case-insensitive project-name uniqueness with HTTP conflict handling
* [x] typed frontend project listing, creation, detail, and branch-listing clients
* [x] authenticated My Projects listing with loading, empty, and retryable-error states
* [x] protected Create Project form with name/description validation and duplicate-name feedback
* [x] real-browser project creation, persistence, and duplicate-name recovery verification
* [x] protected, read-only project overview with owner-scoped metadata
* [x] project overview navigation, loading/error/retry states, and real-browser acceptance
* [x] read-only branch summaries with truthful head-revision states
* [ ] integrated project-file browsing, source-file actions, and CAD preview workspace
* [ ] browser project metadata editing
* [ ] broader project management and collaboration interfaces

### Versioning, Storage, and CAD

* [x] revision and branch persistence foundation
* [x] automatic `main` branch creation
* [x] atomic revision creation with optimistic branch-head updates
* [x] authenticated branch/revision reads and revision-creation API
* [x] authenticated branch creation API
* [x] authenticated branch rename API
* [x] authenticated branch deletion API
* [ ] broader branch management
* [ ] interactive browser branch switching and revision-history navigation
* [x] branch-head DAG traversal and history API
* [x] immutable project-file metadata and content-reference update protection
* [x] transactionally finalized, immutable revision-file snapshots
* [x] immutable revision metadata and ancestry with independent-deletion protection
* [ ] comprehensive revision-graph cycle validation
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
* [x] Three.js browser viewer
* [x] authenticated routed project-file preview states and lazy loading
* [x] STEP/STP-to-GLB conversion through MayoConv 0.10.0
* [x] conversion-job execution deadline and Linux Mayo process-group cancellation
* [ ] conversion support for additional native CAD formats
* [ ] assembly/revision relationships

### Product Evolution

* [ ] public project sharing
* [ ] collaboration and permissions
* [ ] revision-aware visual comparison
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

As a portfolio project, 3Default demonstrates practical engineering across Go, PostgreSQL, SQL and schema design, database-enforced revision immutability, REST APIs, authentication, Docker, generated code workflows, integration testing, content-addressed storage, streaming file uploads and downloads, durable background-job orchestration, concurrency-safe PostgreSQL work claiming, transactional derived-output persistence, authenticated GLB preview streaming, Git, and product-oriented architecture.

As a potential product, the goal is to preserve a foundation that can evolve into a usable engineering collaboration platform without discarding the portfolio implementation and starting over.

Longer-term, 3Default is intended to support version control and visual collaboration across mechanical CAD, printable meshes, creative 3D assets, and game-development models. Its central product goal is to help users identify what changed, where it changed, and which exact revisions contain those changes.

Future comparison workflows should support selecting any two revisions, including revisions from different branches, with side-by-side rendering, synchronized cameras, optional overlays, and eventually format-appropriate geometric differences. Historical previews must be resolved from the exact source-file snapshots referenced by those revisions rather than treating the latest project-file preview as historical evidence.

These are long-term product goals, not currently implemented capabilities or instructions to expand immediate development scope. Original source files remain authoritative; GLB browser previews remain derived artifacts, and conversion adapters remain replaceable.

---

## License

This repository currently has no open-source license.

No license to use, modify, or redistribute the source code is granted except as otherwise permitted by applicable law or GitHub's Terms of Service.

---

## Author

**Angel Aviles**

Software engineer based in Waterloo, Ontario.

GitHub: [github.com/AngelAvilesSil](https://github.com/AngelAvilesSil)
