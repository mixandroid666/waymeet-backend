# waymeet Backend â€” Architecture (for frontend developers)

This doc explains how the backend is built, using ideas you already know from
frontend work. If you've built a React or Flutter app, you'll recognize more of
this than you'd expect â€” the names are different, the ideas are the same.

---

## 1. The big picture

The backend is **one program** that the Flutter app talks to over HTTP (plus a
second small program for slow background work). It is the thing that:

- stores data permanently (users, posts, messages),
- enforces rules (passwords, "are you allowed to do this?"),
- and answers requests from the app.

```
                                  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
  Flutter app  â”€â”€HTTP/JSONâ”€â”€â–¶     â”‚      API program (cmd/api)  â”‚
  (phone)      â—€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€     â”‚   "answer requests fast"    â”‚
                                  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                                              â”‚
                  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
                  â–¼                            â–¼                           â–¼
          â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”            â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”            â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
          â”‚  PostgreSQL  â”‚            â”‚    Redis     â”‚            â”‚  S3 / MinIO  â”‚
          â”‚  + PostGIS   â”‚            â”‚ cache/queue/ â”‚            â”‚  files:      â”‚
          â”‚  the data    â”‚            â”‚ chat fan-out â”‚            â”‚ images,video â”‚
          â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜            â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜            â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                                              â–²
                                  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”´â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
                                  â”‚   Worker program (cmd/worker)â”‚
                                  â”‚  "do slow stuff later"      â”‚
                                  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

**Frontend analogy:** the API is like your app's "backend-for-frontend" that you
always wished was clean. PostgreSQL is your single source of truth (like a giant,
permanent, queryable version of your app state). Redis is fast temporary memory.
S3 is where big files live (you'd never store a video in a database, just like
you'd never put a video file in your Redux store).

---

## 2. Two programs, not one

Look in `cmd/`:

| Program | File | Job | Analogy |
|---|---|---|---|
| **api** | `cmd/api/main.go` | Answers HTTP requests from the app. Must be **fast**. | The part of your app that responds to a button tap instantly. |
| **worker** | `cmd/worker/main.go` | Does slow jobs in the background (resize images, transcode video, send push notifications). | A `Future`/background isolate that runs work off the main thread so the UI never freezes. |

**Why split them?** If resizing a 4K video happened *inside* the API request, the
user's app would hang waiting. Instead the API says "job queued, here you go" and
returns immediately; the worker picks the job up from Redis and grinds on it.

> Right now the worker is a placeholder (an empty loop). It becomes real when the
> media/notification features land.

---

## 3. "Modular monolith" â€” what that means

The README calls this a **modular monolith**. Translation:

- **Monolith** = it's *one* deployable program (the api), not 20 microservices.
  Simple to run, simple to reason about. Good choice for a project this size.
- **Modular** = inside that one program, the code is split into clean **modules**
  that don't reach into each other's internals. Each module owns one feature.

**Frontend analogy:** it's exactly like organizing a React app into
self-contained feature folders (`/features/auth`, `/features/feed`, `/features/chat`)
instead of dumping every component in one folder. One app, clean internal seams.

The modules live in `internal/` and each maps to a screen in the Flutter app:

| Module | Folder | Owns | Flutter screen |
|---|---|---|---|
| **auth** | `internal/auth/` | Sign up, login, OTP, tokens | Login / register |
| **user** | `internal/user/` | Profiles, follow/unfollow | Profile |
| **feed** | `internal/feed/` | Posts, stories, likes, comments | Home feed |
| **location** | `internal/location/` | "Nearby people" (geospatial) | Discover / map |
| **chat** | `internal/chat/` | 1:1 messaging (realtime) | Chat |
| **media** | `internal/media/` | Image/video/voice uploads | (used by feed & chat) |
| **notification** | `internal/notification/` | Push notifications | (background) |

> Today only **auth** is fully built. The others are `doc.go` stubs â€” a single
> file describing what the module *will* do. They're placeholders marking the
> seams, so the structure is decided up front and features slot in cleanly.

---

## 4. The layers inside a module (the important part)

Every feature module is built in **three layers**. This is the single most
important pattern to understand. Look at `internal/auth/` â€” it has exactly these:

```
   HTTP request from the app
            â”‚
            â–¼
   â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
   â”‚  Handler         â”‚   handler.go     "the web layer"
   â”‚  (HTTP in/out)   â”‚
   â””â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
            â”‚  plain function call, plain Go structs
            â–¼
   â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
   â”‚  Service         â”‚   service.go     "the brain / business rules"
   â”‚  (the logic)     â”‚
   â””â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
            â”‚  calls type-safe query methods
            â–¼
   â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
   â”‚  Repository /    â”‚   storage + dbgen  "the data layer"
   â”‚  Queries (SQL)   â”‚
   â””â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
            â”‚
            â–¼
        PostgreSQL
```

### Layer 1 â€” Handler (`handler.go`)
Knows about HTTP and **nothing else**. It:
1. reads the JSON body,
2. validates the shape (is this a valid email? is the password long enough?),
3. calls the Service,
4. turns the result (or error) into an HTTP status + JSON.

**Frontend analogy:** this is your API route handler / controller. It's the
thin glue between "the web" and "your real logic." Notice it never touches the
database directly â€” same discipline as keeping `fetch` calls out of your UI
components.

See `register()` in `handler.go`: it decodes the request, validates, calls
`svc.Register(...)`, and maps the outcome to `201 Created` or an error.

### Layer 2 â€” Service (`service.go`)
The **brain**. All the real rules live here, and it has *no idea* HTTP exists.
Example rules from `auth/service.go`:
- an OTP code expires after 10 minutes,
- you get 5 attempts before lockout,
- you must wait 60 seconds before resending a code,
- a refresh token can only be used once (reuse = someone stole it â†’ revoke all sessions).

**Frontend analogy:** this is like your state-management/business logic layer
(your hooks, your Bloc/Cubit, your service classes) â€” pure logic, testable
without a browser. You could swap HTTP for a CLI and this layer wouldn't change.

### Layer 3 â€” Data access (`internal/platform/storage` + `dbgen/`)
The only layer that talks SQL to PostgreSQL. **This code is generated, not
hand-written** â€” see section 6.

**Why three layers?** Each can change independently. Swap the web framework â†’
only handlers change. Change a business rule â†’ only the service changes. This
separation is *the* reason the README can promise "extract any module into its
own service later without untangling."

---

## 5. Follow one real request end-to-end

Here's what actually happens when the app calls **`POST /api/v1/auth/login`**:

1. **Arrives at the server** (`internal/platform/server/server.go`). The server
   is started in `cmd/api/main.go` and listens on port `8080`.

2. **Passes through middleware** (`internal/platform/httpx/httpx.go`):
   - `Recoverer` â€” if anything crashes, turn it into a clean `500` instead of
     killing the whole server.
   - `Logger` â€” log the method, path, and status of every request.

   **Frontend analogy:** middleware = like Axios interceptors or Express
   middleware. A pipeline every request flows through.

3. **Routed to the handler.** The router matches `POST /api/v1/auth/login` to
   `Handler.login` (registered in `handler.go` â†’ `RegisterRoutes`).

4. **Handler validates** the email/phone + password shape, then calls
   `service.Login(...)`.

5. **Service runs the logic:** look up the user, check the password hash, confirm
   the account is verified, then mint tokens.

6. **Data layer** runs the actual SQL (`GetCredentialsByEmail`) against Postgres.

7. **Response bubbles back up:** Service â†’ Handler â†’ JSON `200 OK` with an
   `access_token` + `refresh_token`. The app stores those and attaches the access
   token to future requests.

```
POST /api/v1/auth/login
  â†’ Recoverer â†’ Logger          (middleware pipeline)
  â†’ Handler.login               (validate JSON)
  â†’ Service.Login               (check password, account status)
  â†’ Queries.GetCredentialsByEmail   (SQL â†’ Postgres)
  â† TokenPair                   (access + refresh tokens)
  â† 200 OK {access_token, refresh_token, user}
```

---

## 6. How the database code works (sqlc â€” the cool part)

Most backends either hand-write SQL strings (error-prone) or use a heavy ORM
(magic, slow). This project does something nicer: **sqlc**.

You write plain SQL once, in `db/queries/*.sql`, like:

```sql
-- name: GetCredentialsByEmail :one
SELECT id, status, password_hash FROM users WHERE email = $1;
```

Then run `make sqlc`, and it **generates type-safe Go functions** for you into
`internal/platform/storage/dbgen/`. Now the service just calls
`Queries.GetCredentialsByEmail(ctx, email)` and gets back a typed struct.

**Frontend analogy:** it's like GraphQL Code Generator or `openapi-typescript` â€”
you define the contract once, and it generates fully-typed client functions so
you can't typo a field or pass the wrong type. If your SQL and your Go disagree,
it won't compile. You never hand-write the data layer.

> âš ï¸ Don't edit files in `dbgen/` by hand â€” they're regenerated and your changes
> would be wiped. Edit the `.sql` files in `db/queries/` and run `make sqlc`.

### Migrations â€” versioning the database shape
The database's structure (tables, columns) is itself versioned, in
`db/migrations/`. Each change is a numbered pair:

- `0001_init.up.sql` â€” apply the change (create tables)
- `0001_init.down.sql` â€” undo it

`make migrate-up` applies them in order. **Frontend analogy:** it's `git` for
your database schema â€” an ordered, replayable history so every environment
(your laptop, the cloud) ends up with the exact same tables.

---

## 7. Where things are stored (and why three different places)

| Store | Holds | Why not just use Postgres? |
|---|---|---|
| **PostgreSQL** | Users, posts, messages, tokens â€” the permanent truth | It's the source of truth. Reliable, queryable, transactional. |
| **Redis** | Cache, the job queue, chat message fan-out | Fast but temporary. Great for "ephemeral + high-volume." Losing it is survivable. |
| **S3 / MinIO** | The actual image/video/voice **files** | Databases are terrible at big binary blobs. Files go in object storage; the DB just stores the *URL*. |

**The PostGIS detail:** Postgres has a geospatial extension called PostGIS. It's
why the "nearby people" feature is possible â€” it can answer "find users within
5km of this point" efficiently. That single requirement is why the stack is
Postgres and not, say, MongoDB.

**The upload trick (`media` module):** when a user uploads a video, the bytes
do **not** flow through the API. Instead the API hands the app a temporary
"presigned URL," and the app uploads *directly* to S3. This keeps the API fast
and cheap. (Think: getting a signed upload URL from your backend, then PUTting
the file straight to the bucket.)

---

## 8. Authentication â€” how "being logged in" works

This is fully built, and it's a great example of the patterns above. Two token
types:

- **Access token** (JWT, short-lived â€” 15 min): sent on every request in the
  `Authorization: Bearer <token>` header. It's self-contained: the server can
  verify it without a database lookup.
- **Refresh token** (long-lived â€” 30 days, stored in the DB): used *only* to get
  a new access token when the old one expires.

**Frontend analogy:** exactly the access/refresh pattern you've implemented on
the client side â€” short token for requests, refresh token to stay logged in
without re-entering a password.

**Protecting a route:** `auth.Service.Middleware` wraps any handler so it
rejects requests without a valid access token, and makes the user's ID available
to the handler. See `GET /api/v1/auth/me` â€” a one-line example of a protected
endpoint.

Security niceties already handled: passwords and OTP codes are **hashed** (never
stored in plain text), and refresh tokens **rotate** â€” using one invalidates it
and issues a fresh one. If an old (already-used) refresh token shows up, that
signals theft, so *all* the user's sessions get revoked.

---

## 9. The `platform/` folder â€” shared plumbing

`internal/platform/` is cross-cutting infrastructure that every module reuses,
so the rules are written once:

| Folder | Job |
|---|---|
| `config/` | Reads settings from environment variables (DB URL, secrets, etc.) |
| `logging/` | Sets up structured logging (`slog`) |
| `httpx/` | JSON responses, the standard error envelope, middleware |
| `server/` | Builds the HTTP server and registers every module's routes |
| `storage/` | The Postgres connection pool + the generated query layer |

**Frontend analogy:** this is your `/lib`, `/utils`, `/core` â€” shared helpers and
config that aren't tied to any one feature.

### Configuration via environment variables
The app reads all its settings from the environment (`config/config.go`), never
hard-coded. `DATABASE_URL`, `JWT_SECRET`, `REDIS_URL`, the S3 keys â€” all injected
from outside. `.env.example` lists them. **Frontend analogy:** identical to your
`.env` / `VITE_*` / `dart-define` variables. Same code, different config per
environment (laptop vs. cloud).

---

## 10. Cheat sheet: frontend term â†’ backend term

| You know (frontend) | Here it's called | Where |
|---|---|---|
| API route / controller | Handler | `*/handler.go` |
| Business logic / Bloc / service class | Service | `*/service.go` |
| Typed API client (GraphQL codegen) | sqlc-generated queries | `storage/dbgen/` |
| `.env` / build-time config | Env config | `platform/config/` |
| Axios/Express interceptors | Middleware | `platform/httpx/` |
| Redux store (app state) | PostgreSQL (persistent) / Redis (ephemeral) | â€” |
| Background isolate / Web Worker | The worker program | `cmd/worker/` |
| Feature folders | Modules | `internal/<module>/` |
| `git` for your DB schema | Migrations | `db/migrations/` |

---

## 11. Where to start reading the code

In this order, you'll understand the whole thing in ~30 minutes:

1. `cmd/api/main.go` â€” where the program starts (~60 lines).
2. `internal/platform/server/server.go` â€” how requests get routed.
3. `internal/auth/handler.go` â€” the web layer of a real feature.
4. `internal/auth/service.go` â€” the brain of a real feature.
5. `db/queries/auth.sql` + `db/migrations/0002_auth.up.sql` â€” the data shape.

Everything else is a variation on what you'll see in those five files.
```
