# Package Oriented Design — Go Project Layout

## The Core Principle

> **A package is not a namespace. A package is a unit of design.**

This is the defining difference between Package Oriented Design and how Java/C# engineers approach Go. In Java, packages are folders that group related things. In Go, **a package defines what it IS** — its purpose, its data, and its behavior are inseparable.

The question is never *"where should I put this file?"*  
The question is *"what single concept does this package represent?"*

---

## What a Package IS (and is NOT)

### A Package IS:
- A single, self-contained concept with a clear boundary
- The owner of its own data — it does not share raw types with other packages
- Named after what it represents, not what it does
- Small enough that its entire API can be understood in one reading

### A Package is NOT:
- A folder to dump related files
- A Java-style namespace (`com.company.utils`)
- A collection of helpers for other packages
- An excuse to avoid thinking about design

### The Naming Test
If you cannot name your package with a **single, specific noun**, the package has no identity yet.

| Bad Name | Why It Fails | What You're Really Designing |
|---|---|---|
| `util` | It's a drawer, not a concept | Break into specific packages |
| `helper` | Helps what? | A package that helps nothing |
| `common` | Common to whom? | Shared coupling disguised as reuse |
| `types` | Types of what? | An anemic data layer |
| `models` | ORM thinking | Domain types belong inside their domain package |
| `base` | Base of what? | Missing abstraction |
| `shared` | Red flag: forced coupling | Rethink the boundaries |
| `manager` | Manages what? | A God object disguised as a package |

---

## Canonical Project Structure

```
myservice/
├── cmd/
│   └── myservice/
│       └── main.go          ← Thin: wires dependencies, calls Run(), exits
├── internal/
│   ├── user/                ← The "user" concept owns its data AND behavior
│   │   ├── user.go          ← Type definition + constructors
│   │   ├── store.go         ← Persistence interface + implementation
│   │   ├── handler.go       ← HTTP handlers for user endpoints
│   │   └── user_test.go
│   ├── auth/                ← The "auth" concept: tokens, sessions, claims
│   │   ├── auth.go
│   │   ├── token.go
│   │   └── middleware.go
│   ├── platform/            ← Cross-cutting infrastructure (not "util")
│   │   ├── postgres/        ← DB connection + pool setup
│   │   ├── logger/          ← Logger initialization
│   │   └── config/          ← Config loading and validation
│   └── app/                 ← Wires everything together; the application graph
│       └── app.go
├── go.mod
└── go.sum
```

### `cmd/` — Entry Points Only
Each subdirectory of `cmd/` is a `main` package. Its job is **exclusively** to:
1. Parse flags / environment
2. Build the dependency graph
3. Call the top-level `Run(ctx)` function
4. Handle the exit code

If `main.go` is longer than ~50 lines, business logic has leaked into it.

```go
// cmd/myservice/main.go — this is ALL it should do
func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config: %v", err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    if err := app.Run(ctx, cfg); err != nil {
        log.Fatalf("run: %v", err)
    }
}
```

### `internal/` — The Compiler-Enforced Boundary
`internal/` is **not a convention** — it is enforced by the Go compiler. Code inside `internal/` can only be imported by code rooted at the parent of `internal/`. External packages and other modules cannot import it.

Use `internal/` for:
- All business logic that should not be a public API
- Domain packages (`user`, `order`, `payment`)
- Infrastructure wrappers (`platform/postgres`, `platform/redis`)

Use a public `pkg/` only for code you explicitly want other modules to import. When in doubt, `internal/` first.

---

## Package Oriented Design in Practice

### Each Domain Package Owns Its World

The `user` package owns the `User` type. Not `models.User`. Not `types.User`. The type, the persistence interface, the validation, and the HTTP handlers all live together because they are all about the same concept.

```
internal/user/
├── user.go      ← type User struct { ... }  +  constructors
├── store.go     ← type Store interface { ... }  +  PostgresStore implementation
├── handler.go   ← HTTP handlers that speak "user" language
└── user_test.go ← tests for all of the above
```

```go
// internal/user/user.go
package user

// User is the central type of this package.
// Every other file in this package exists to serve this concept.
type User struct {
    ID        int64
    Email     string
    CreatedAt time.Time
}

// New is the only way to create a valid User — constructors enforce invariants.
func New(email string) (*User, error) {
    if !isValidEmail(email) {
        return nil, fmt.Errorf("user: invalid email %q", email)
    }
    return &User{Email: email, CreatedAt: time.Now()}, nil
}
```

```go
// internal/user/store.go
package user

// Store defines what persistence means for the user concept.
// Defined HERE, at the point of use — not in a separate "interfaces" package.
type Store interface {
    GetByID(ctx context.Context, id int64) (*User, error)
    Create(ctx context.Context, u *User) error
    Delete(ctx context.Context, id int64) error
}

// PostgresStore is the production implementation.
type PostgresStore struct{ db *pgxpool.Pool }

func (s *PostgresStore) GetByID(ctx context.Context, id int64) (*User, error) { ... }
```

```go
// internal/user/handler.go
package user

// Handler speaks HTTP but thinks in users.
// It depends on the Store interface — not on PostgresStore.
type Handler struct{ store Store }

func NewHandler(s Store) *Handler { return &Handler{store: s} }

func (h *Handler) ServeGetUser(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
    u, err := h.store.GetByID(r.Context(), id)
    if errors.Is(err, ErrNotFound) {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }
    // ...
}
```

### Platform Packages — Not "util"

Infrastructure code that multiple domain packages need is NOT `util/`. It is a **specific named concept**:

```
internal/platform/
├── postgres/    ← package postgres: opens pool, runs health check, returns *pgxpool.Pool
├── logger/      ← package logger: builds *slog.Logger from config
├── config/      ← package config: loads and validates Config struct
└── httpserver/  ← package httpserver: builds *http.Server with timeouts, graceful shutdown
```

```go
// internal/platform/postgres/postgres.go
package postgres

// Open returns a configured, healthy connection pool.
// It encapsulates all the pool-tuning knowledge so callers don't need to know it.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, fmt.Errorf("postgres: parse config: %w", err)
    }
    cfg.MaxConns = 25
    cfg.MinConns = 5
    cfg.MaxConnLifetime = 5 * time.Minute
    cfg.MaxConnIdleTime = 1 * time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("postgres: open: %w", err)
    }
    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("postgres: ping: %w", err)
    }
    return pool, nil
}
```

### `app/` — The Dependency Graph

`app.go` is the only place in the codebase that knows about all packages simultaneously. Its job is to wire the graph and nothing else.

```go
// internal/app/app.go
package app

func Run(ctx context.Context, cfg *config.Config) error {
    // Infrastructure
    pool, err := postgres.Open(ctx, cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("app: %w", err)
    }
    defer pool.Close()

    log := logger.New(cfg.LogLevel)

    // Domain — dependencies flow inward (domain never imports platform directly)
    userStore  := &user.PostgresStore{Pool: pool}
    userHandler := user.NewHandler(userStore)

    authStore  := &auth.PostgresStore{Pool: pool}
    authMW     := auth.NewMiddleware(authStore)

    // HTTP
    mux := http.NewServeMux()
    mux.Handle("/users/{id}", authMW(http.HandlerFunc(userHandler.ServeGetUser)))

    srv := httpserver.New(cfg.Addr, mux)
    return srv.Run(ctx) // blocks until ctx is cancelled
}
```

---

## Anti-Patterns — Spotted in the Wild

### The God Package
```
// BAD: one package that knows everything
internal/service/
├── user_service.go
├── auth_service.go
├── payment_service.go
└── email_service.go  // ← not a "service", it's everything
```

### The Anemic Type Layer
```
// BAD: types separated from their behavior — Java DTO thinking
internal/models/
└── models.go  // type User struct { ... }  type Order struct { ... }

internal/repositories/
└── user_repo.go  // func GetUser(id int64) (*models.User, error)

internal/services/
└── user_service.go // func CreateUser(u *models.User) error
```
The `User` type is owned by nobody. Three packages must coordinate to do anything with a user.

### The Circular Import Smell
If packages A and B need to import each other, one of three things is true:
1. They are actually one concept and should be merged
2. A third package C needs to be extracted that both A and B depend on
3. An interface should replace the direct dependency

---

## Rules Summary

1. **Name packages after what they represent, not what they do**
2. **Each package owns its types** — no shared `models/` or `types/` package
3. **Define interfaces at the point of use** — inside the package that depends on them, not inside the package that implements them
4. **`internal/` first** — expose a public API only when another module needs it
5. **`cmd/` packages are wiring only** — 50 lines max, no business logic
6. **Platform packages are named concepts** — `postgres`, `redis`, `logger`, never `util`
7. **If you can't name it, you haven't designed it yet**
