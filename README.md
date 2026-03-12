# sqlgen

Database-first code generator for Go. Point it at your DDL files or a live database and it spits out type-safe models, CRUD operations, query builders, and relationship loading.

Supports **Postgres**, **MySQL**, and **SQLite**. No cgo required.

Inspired by [SQLBoiler](https://github.com/volatiletech/sqlboiler), [Bob](https://github.com/stephenafamo/bob), and [jOOQ](https://www.jooq.org/), rebuilt from scratch.

### Lineage

**jOOQ** (Java) pioneered the database-first, generated-code approach to query building. sqlgen borrows several of its core ideas:

- **Schema drives the code.** jOOQ reads your database schema and generates Java classes. sqlgen does the same for Go, from either DDL files or a live database connection.
- **Per-column type-safe predicates.** jOOQ generates `USERS.EMAIL.eq("x")`. sqlgen generates `UserWhere.Email.EQ("x")`. Same concept, same compile-time safety, different syntax shaped by Go's type system.
- **Generated metadata objects.** jOOQ gives you `Table` and `Field` references for every schema object. sqlgen generates `UserColumns.Email`, `UserTableName`, and typed filter structs that serve the same purpose.
- **Composable query building.** jOOQ chains methods; sqlgen composes `QueryMod` functions. Both let you build queries piece by piece without string concatenation.
- **The database is the source of truth.** Both reject the "code defines schema" ORM pattern. Your tables, types, and constraints are defined in SQL, and the generated code reflects them exactly.

Where sqlgen diverges: jOOQ is a full query DSL that covers nearly all of SQL. sqlgen is more opinionated, generating CRUD and relationship loading with a thinner query builder. jOOQ targets Java (and Kotlin/Scala); sqlgen targets Go. And while jOOQ requires a JDBC connection, sqlgen supports both live database introspection *and* offline DDL parsing, with no cgo dependency.

## Install

```bash
go install github.com/davidbyttow/sqlgen/cmd/sqlgen@latest
```

Requires Go 1.23+. Pure Go, no cgo needed. The Postgres DDL parser uses [go-pgquery](https://github.com/wasilibs/go-pgquery) (WebAssembly-based). MySQL and SQLite use hand-written parsers.

## Quick Start

Two ways to feed sqlgen your schema: DDL files (no database required) or a live Postgres connection.

### Option A: From DDL files

1. Write your schema in plain SQL:

```sql
CREATE TYPE post_status AS ENUM ('draft', 'published', 'archived');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    bio TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    status post_status NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
```

2. Create `sqlgen.yaml`:

```yaml
input:
  dialect: postgres    # or "mysql" or "sqlite"
  paths:
    - schema.sql

output:
  dir: models
  package: models
```

3. Generate:

```bash
sqlgen generate
```

That's it. You'll get a `models/` directory with fully typed Go code.

### Option B: From a live database

Point sqlgen at a running Postgres instance. It queries `information_schema` and `pg_catalog` to build the same IR that DDL parsing produces, so the generated code is identical either way.

1. Create `sqlgen.yaml`:

```yaml
input:
  dialect: postgres
  dsn: ${DATABASE_URL}

output:
  dir: models
  package: models
```

The DSN supports environment variable expansion, so you can keep credentials out of the config file.

2. Generate:

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
sqlgen generate
```

See `examples/introspect/` for a working example.

## Supported Dialects

| Dialect | DDL Parsing | Live Introspection | Parser |
|---------|:-----------:|:------------------:|--------|
| **Postgres** | ✅ | ✅ | [go-pgquery](https://github.com/wasilibs/go-pgquery) (Wasm, no cgo) |
| **MySQL** | ✅ | ❌ | Hand-written, zero deps |
| **SQLite** | ✅ | ❌ | Hand-written, zero deps |

All 3 dialects produce the same schema IR, so the generated Go code is structurally identical regardless of which database you're using.

### MySQL-specific notes

- `ENUM('val1','val2')` column types are extracted and generated as type-safe Go enums
- Backtick-quoted identifiers are handled automatically
- `UNSIGNED` integer types map to Go unsigned types (`uint32`, `uint64`, etc.)
- `TINYINT(1)` is treated as `bool`
- Table options (`ENGINE`, `CHARSET`, etc.) are parsed and discarded

### SQLite-specific notes

- `INTEGER PRIMARY KEY` is recognized as auto-increment (ROWID alias)
- Type affinity rules are followed: `VARCHAR(255)` maps to `string`, `REAL` maps to `float64`, etc.
- No enum support (SQLite doesn't have enums)

## Generated API

### Models

Each table becomes a Go struct. Column types map to their Go equivalents. Nullable columns use `runtime.Null[T]` by default.

```go
type User struct {
    ID        string          `json:"id" db:"id"`
    Email     string          `json:"email" db:"email"`
    Name      string          `json:"name" db:"name"`
    Bio       runtime.Null[string] `json:"bio" db:"bio"`
    CreatedAt time.Time       `json:"created_at" db:"created_at"`

    R *UserRels `json:"-" db:"-"`
}
```

Every model also gets:
- `UserTableName` constant (`"users"`)
- `UserColumns` struct with column name constants
- `UserSlice` type alias (`[]*User`)
- `ScanRow` method for scanning from `*sql.Row` or `*sql.Rows`

### Column Constants

Safe column name references, useful for building queries or referencing column names without string literals:

```go
models.UserColumns.ID        // "id"
models.UserColumns.Email     // "email"
models.UserColumns.CreatedAt // "created_at"
```

### Type-Safe Where Clauses

Each table gets a `<Model>Where` variable with per-column filter builders. These return `runtime.QueryMod` values you can compose freely.

```go
q := models.Users(
    models.UserWhere.Email.EQ("alice@example.com"),
    runtime.Limit(1),
)

sql, args := q.BuildSelect()
// SELECT "id", "email", "name", "bio", "created_at"
//   FROM "users" WHERE "email" = $1 LIMIT 1
// args: ["alice@example.com"]
```

Available filter methods per column:
- `EQ`, `NEQ`, `LT`, `LTE`, `GT`, `GTE`
- `IN` (variadic)
- `IsNull`, `IsNotNull` (nullable columns only)

### Composing Queries

Stack multiple mods. WHERE clauses are ANDed together.

```go
q := models.Posts(
    models.PostWhere.Status.EQ(models.PostStatusPublished),
    models.PostWhere.AuthorID.EQ("some-uuid"),
    runtime.OrderBy("published_at DESC"),
    runtime.Limit(10),
    runtime.Offset(20),
)
```

Other available mods: `GroupBy`, `Having`, `Join`, `LeftJoin`, `ForUpdate`, `WithCTE`.

### CTEs (WITH Clause)

Common Table Expressions for complex queries, including recursive CTEs for hierarchical data:

```go
// Simple CTE
q := models.Users(
    runtime.WithCTE("active", "SELECT * FROM users WHERE active = ?", true),
    runtime.Where(`"id" IN (SELECT id FROM active)`),
)

// Recursive CTE (e.g., category tree)
q := runtime.NewQuery(dialect, "tree",
    runtime.WithRecursiveCTE("tree",
        "SELECT id, parent_id, name FROM categories WHERE parent_id IS NULL "+
        "UNION ALL "+
        "SELECT c.id, c.parent_id, c.name FROM categories c JOIN tree t ON c.parent_id = t.id"),
)
```

### Row Locking

Pessimistic locking for transactional workflows:

```go
// FOR UPDATE (exclusive lock)
q := models.Users(
    models.UserWhere.ID.EQ("some-uuid"),
    runtime.ForUpdate(),
)

// FOR UPDATE NOWAIT (fail immediately if locked)
q := models.Users(
    models.UserWhere.ID.EQ("some-uuid"),
    runtime.ForUpdate(),
    runtime.Nowait(),
)

// FOR UPDATE SKIP LOCKED (skip locked rows, useful for job queues)
q := models.Users(
    runtime.ForUpdate(),
    runtime.SkipLocked(),
    runtime.Limit(1),
)
```

Four lock strengths: `ForUpdate()`, `ForShare()`, `ForNoKeyUpdate()`, `ForKeyShare()`.

### CRUD Operations

Generated functions for each table:

```go
// Query builders
models.Users(mods...)            // SELECT with mods
models.FindUser(ctx, db, id)     // Find by primary key
models.AllUsers(ctx, db)         // SELECT all rows
models.UserExists(ctx, db, id)   // Returns bool
models.UserCount(ctx, db)        // COUNT(*)

// Mutations
user.Insert(ctx, db)   // INSERT with RETURNING
user.Update(ctx, db)   // UPDATE by PK
user.Delete(ctx, db)   // DELETE by PK
user.Upsert(ctx, db)   // INSERT ON CONFLICT DO UPDATE
```

All mutations accept a `context.Context` and a `runtime.Executor` (which `*sql.DB` and `*sql.Tx` both satisfy).

#### Partial Mutations (Whitelist/Blacklist)

Control which columns are included in Insert, Update, or Upsert:

```go
// Only update these columns:
user.Update(ctx, db, runtime.Whitelist("email", "name"))

// Update everything except these:
user.Update(ctx, db, runtime.Blacklist("created_at"))

// Partial insert:
user.Insert(ctx, db, runtime.Whitelist("email", "name"))
```

### Streaming Iteration

For large result sets where you don't want to load everything into memory:

```go
// Callback style: process one row at a time
err := models.EachUser(ctx, db, func(u *models.User) error {
    fmt.Println(u.Email)
    return nil
}, runtime.Where(`"active" = ?`, true))

// Cursor style: manual iteration with explicit close
cursor, err := models.UserCursor(ctx, db, runtime.OrderBy(`"created_at" DESC`))
if err != nil { ... }
defer cursor.Close()

for user, ok := cursor.Next(); ok; user, ok = cursor.Next() {
    fmt.Println(user.Email)
}
if err := cursor.Err(); err != nil { ... }
```

### Enums

SQL enums become type-safe Go string types:

```go
status := models.PostStatusDraft     // "draft"
status.IsValid()                     // true
status.String()                      // "draft"
models.AllPostStatusValues()         // []PostStatus{"draft", "published", "archived"}

// Implements sql.Scanner and driver.Valuer for DB round-tripping.
```

### Hooks

Register typed lifecycle hooks per model. Hooks receive the model pointer, so you can inspect or modify the row before it hits the database.

```go
models.AddUserHook(runtime.BeforeInsert, func(ctx context.Context, exec runtime.Executor, user *models.User) (context.Context, error) {
    log.Printf("inserting user: %s", user.Email)
    return ctx, nil
})
```

9 hook points: `BeforeInsert`, `AfterInsert`, `BeforeUpdate`, `AfterUpdate`, `BeforeDelete`, `AfterDelete`, `BeforeUpsert`, `AfterUpsert`, `AfterSelect`.

Skip hooks on a per-call basis via context:

```go
ctx := runtime.SkipHooks(context.Background())
user.Insert(ctx, db) // hooks won't fire
```

Disable hook generation entirely with `output.no_hooks: true` in your config.

### Automatic Timestamps

sqlgen auto-manages `created_at` and `updated_at` columns when they exist on a table. On `Insert`, both get set to `time.Now()`. On `Update`, `updated_at` gets refreshed.

Column names are configurable (or disable with `"-"`):

```yaml
timestamps:
  created_at: created_at   # default
  updated_at: updated_at   # default, or "-" to disable
```

### Relationships

sqlgen infers relationships from foreign keys:

- **BelongsTo**: `posts.author_id -> users.id` gives `Post.R.User`
- **HasMany**: inverse of the above gives `User.R.Posts`
- **HasOne**: FK with a unique constraint
- **ManyToMany**: detected via join tables (composite PK, 2 FKs, no extra columns)

Relationship fields live on the `R` struct:

```go
type UserRels struct {
    Posts []*Post  // HasMany
}

type PostRels struct {
    User *User     // BelongsTo
    Tags []*Tag    // ManyToMany (via post_tags)
}
```

### Eager Loading

Two strategies for loading relationships:

**Preload (LEFT JOIN, single query)** for to-one relationships (BelongsTo, HasOne):

```go
// Loads posts with their author in a single query via LEFT JOIN.
posts, _ := models.AllPosts(ctx, db, runtime.Preload(models.PostPreloadUser))
posts[0].R.User.Email // already populated, no extra query
```

**LoadRelations (separate batch queries)** for all relationship types:

```go
posts, _ := models.AllPosts(ctx, db)
posts.LoadRelations(ctx, db, runtime.Load("User"), runtime.Load("Tags"))
```

Supports dot-notation nesting and filtered loading:

```go
users.LoadRelations(ctx, db,
    runtime.Load("Posts.Tags"),
    runtime.Load("Posts", runtime.Where(`"status" = ?`, "published")),
)
```

### Null Types

The `runtime.Null[T]` generic type wraps nullable columns:

```go
user := models.User{
    Bio: runtime.NewNull("Writes Go."),
}

user.Bio.Valid    // true
user.Bio.Val      // "Writes Go."
user.Bio.Ptr()    // *string pointing to "Writes Go."
user.Bio.Clear()  // sets Valid = false

// JSON: marshals to value or null
// SQL: implements Scanner and Valuer
```

### Factories

When `output.factories: true`, sqlgen generates factory functions for each table. Useful for tests.

```go
// Create a user with random values for all non-auto-increment fields.
user := models.NewUser()

// Override specific fields with modifier functions.
user := models.NewUser(func(u *models.User) {
    u.Email = "alice@example.com"
})

// Create and insert in one shot.
user, err := models.InsertUser(ctx, db, func(u *models.User) {
    u.Name = "Alice"
})
```

Random values come from `runtime/fake` (pure Go, no external deps).

### DB Error Matching

Generated constraint constants and runtime matchers for Postgres errors:

```go
if runtime.IsUniqueViolation(err) {
    // handle duplicate
}

if runtime.IsConstraintViolation(err, models.UsersEmailKey) {
    // handle specific constraint
}
```

Works with both `pgx` and `lib/pq` without importing either (reflection-based).

### Prepared Statement Caching

Wrap your executor to automatically prepare and cache statements:

```go
cached := runtime.NewCachedExecutor(db)
defer cached.Close()

// First call prepares; subsequent calls reuse the prepared statement.
user.Insert(ctx, cached)
```

### Bind to Arbitrary Struct

Scan query results into any struct, not just generated models:

```go
type UserSummary struct {
    Name  string `db:"name"`
    Count int    `db:"post_count"`
}

var summaries []UserSummary
err := runtime.Bind(ctx, db, "SELECT name, COUNT(*) as post_count FROM users JOIN posts ...", &summaries)
```

## Configuration

Full `sqlgen.yaml` reference:

```yaml
input:
  dialect: postgres          # "postgres", "mysql", or "sqlite"

  # Option A: parse DDL files (no database needed)
  paths:                     # SQL files or directories
    - schema.sql
    - migrations/

  # Option B: connect to a live database (mutually exclusive with paths)
  # dsn: ${DATABASE_URL}

output:
  dir: models                # output directory
  package: models            # Go package name
  tests: false               # generate _test.go files alongside models
  no_hooks: false            # skip hook generation and hook calls in CRUD
  factories: false           # generate NewX()/InsertX() factory functions
  templates: ""              # path to custom template directory (overlay)

types:
  null: generic              # "generic" (Null[T]), "pointer" (*T), or "database" (sql.NullString)
  replacements:              # override DB type -> Go type
    uuid: "github.com/google/uuid.UUID"
    jsonb: "encoding/json.RawMessage"
  column_replacements:       # override by table.column or *.column
    users.metadata: "map[string]any"
    "*.external_id": "github.com/google/uuid.UUID"

timestamps:
  created_at: created_at     # column name, or "-" to disable
  updated_at: updated_at     # column name, or "-" to disable

tables:
  audit_logs:
    skip: true               # exclude from generation
  users:
    name: Account            # override struct name
    columns:
      email:
        name: EmailAddress   # override field name
        type: "net/mail.Address"  # override Go type

# polymorphic:               # define polymorphic relationships
#   - table: comments
#     type_column: commentable_type
#     id_column: commentable_id
#     targets:
#       User: users
#       Post: posts
```

### Null Type Strategies

Three options for how nullable columns are represented:

| Strategy | Null column | Example |
|----------|-------------|---------|
| `generic` (default) | `runtime.Null[T]` | `Bio runtime.Null[string]` |
| `pointer` | `*T` | `Bio *string` |
| `database` | `sql.NullXxx` | `Bio sql.NullString` |

## Watch Mode

Re-generate automatically when your SQL files change:

```bash
sqlgen watch
```

Uses fsnotify with 200ms debounce. Watches all `.sql` files referenced in your config.

## How It Works

1. **Parse**: DDL files are parsed into a schema IR. Postgres uses go-pgquery (the PostgreSQL parser compiled to WebAssembly). MySQL and SQLite use hand-written parsers. Alternatively, a live Postgres database is introspected via `information_schema` and `pg_catalog`.
2. **Schema IR**: The parsed result is converted to an intermediate representation (tables, columns, constraints, enums, views)
3. **Resolve**: Foreign keys are walked to infer relationships (belongs-to, has-many, has-one, many-to-many)
4. **Generate**: Go templates produce type-safe code for each table, enum, and relationship
5. **Format**: goimports cleans up the output

Generated files are prefixed with `sqlgen_` and contain a `DO NOT EDIT` header. When you drop a table from your DDL, the corresponding generated files get cleaned up automatically on the next run.

## Project Structure

```
sqlgen/
  cmd/sqlgen/       CLI (generate, watch commands)
  schema/           Schema IR types and DDL parsing
    postgres/       Postgres parser (go-pgquery, Wasm-based)
    mysql/          MySQL parser (hand-written)
    sqlite/         SQLite parser (hand-written)
  gen/              Code generation engine and templates
  runtime/          Minimal runtime library imported by generated code
    fake/           Random value generators for factories
  config/           YAML config parsing
  internal/         Naming utilities (case conversion, pluralization)
```

## Status

v1. Postgres, MySQL, SQLite. Go only. Pure Go, no cgo.

Planned:
- Custom query support (name a `.sql` file, get a type-safe Go function)
- More target languages
