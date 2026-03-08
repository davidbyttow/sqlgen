# sqlgen: Research and Design Direction

## The Problem

The Go ecosystem doesn't have a great database-first code generator. SQLBoiler pioneered the approach but is in maintenance mode. Bob picked up where it left off but is pre-1.0 and has rough edges. sqlc is popular but can't handle dynamic queries. GORM is everywhere but slow and magic-heavy.

There's a gap for a tool that combines the best ideas from all of these, built for 2026 and beyond.

## SQLBoiler: The Original

**What it is:** Database-first ORM generator. Connect to a live DB, introspect the schema, generate a full Go ORM package. No reflection, full compile-time type safety, ActiveRecord-like productivity.

**Architecture:**
- Generation pipeline flows through `boilingcore`: config -> driver -> schema introspection -> template rendering -> goimports -> done
- Drivers are separate binaries communicating via JSON over stdin/stdout (clever, but adds complexity)
- Templates use Go's `text/template` with two categories: per-table templates and singleton templates
- Runtime dependencies live in `boil/` and `queries/` packages

**What it gets right:**
- The core bet is correct: generate everything upfront, get type safety for free
- Query mods are composable and intuitive: `models.Users(Where("age > ?", 30), OrderBy("name")).All(ctx, db)`
- Hooks (before/after insert, update, delete) are useful
- Column whitelisting/blacklisting on CRUD operations
- Auto timestamps and soft deletes (convenience features people actually want)
- The driver plugin system means adding new databases doesn't touch core code
- 7k stars, battle-tested in production for years

**What's broken:**
- Eager loading "desperately needs a rewrite" (maintainer's words). Works but has rough edges everywhere
- No multi-column foreign keys
- No cross-schema relationships
- Generated tests are fragile (FK ordering issues, parallel execution problems)
- Doesn't clean up stale files when tables are dropped
- No materialized view support
- All tables must have primary keys
- Shared query object across dialects means you can build invalid queries (JOIN on DELETE compiles fine)
- In maintenance mode since ~2021. Bug fixes only, no new features

**Why it stalled:** The eager loading problem required a rewrite that would break the API. Without a clean path to v5, development stopped. The maintainer recommends Bob or sqlc for new projects.

## Bob: The Successor

**What it is:** Bob is what SQLBoiler v5 could have been. Same philosophy (generate from live DB), but rebuilt from scratch by Stephen Afamo (a SQLBoiler maintainer) without backward compatibility constraints.

**Architecture:**
- 4-layer progressive adoption: query builder -> ORM codegen -> factory generation -> SQL query codegen
- Each layer is independently usable
- Plugin-based template system (models, factories, enums, loaders, joins, counts, queries)
- Dialect-specific query builders (each dialect is hand-crafted, not shared)

**What it fixes over SQLBoiler:**
- Type-safe query types per dialect (can't build invalid queries)
- Cross-schema generation
- Multi-column foreign key support
- Related-through (multi-table relationships)
- Related-where (conditional relationships based on column values)
- Two eager loading strategies: LEFT JOIN preload (to-one) and separate-query ThenLoad (to-one and to-many)
- Hook context chaining (hooks receive and return `context.Context`)
- Factory generation for tests (inspired by Ruby's FactoryBot)
- sqlc-like SQL query codegen layer

**What it drops (intentionally):**
- No automatic timestamps (says do it in the DB)
- No soft deletes (says do it in the DB)

**Pain points:**
- Pre-1.0 (v0.42.0). Breaking changes between releases
- Documentation has gaps; setup instructions unclear for newcomers
- Setter pattern with `omit.Val` is verbose compared to SQLBoiler
- LEFT JOIN preload limited to to-one relationships only
- Circular references in factories cause stack overflow
- Some Postgres types (arrays, JSONB in SQLite) not fully supported
- 1.6k stars vs SQLBoiler's 7k; smaller community, fewer battle scars

## The Broader Landscape

### Other Go Tools Worth Noting

| Tool | Approach | Stars | Strengths | Weaknesses |
|------|----------|-------|-----------|------------|
| **sqlc** | Generate from SQL queries | 17k | Simple, fast, SQL-native | No dynamic queries at all |
| **ent** | Code-first graph schema | 16.9k | Great for complex relationships | Massive codegen, slow compiles |
| **GORM** | Runtime reflection ORM | 39.6k | Easy to start | 4-5x slower, magic-heavy, painful at scale |
| **Jet** | Type-safe query builder + codegen | 2.6k | Excellent type safety, underrated | Small community |
| **Bun** | Lightweight struct-based ORM | 4k | Simple, fast | Less type safety |

### Cross-Language Inspiration

**jOOQ (Java)** is the gold standard for schema-first codegen:
- Connects to live DB or parses DDL files (no running DB required for CI)
- Type-safe DSL that mirrors SQL closely ("what you build is what gets sent")
- Supports Java, Kotlin, Scala output
- Commercially maintained, deeply loved
- Key insight: you should still think in SQL, the tool just adds compile-time safety

**Prisma (TypeScript):**
- Own `.prisma` schema language (not DB-first, but schema-first)
- Prisma 7.0 dropped the Rust engine, went pure TS, got 3x faster
- The schema language is genuinely good; migrations are integrated
- Shows the value of a dedicated schema DSL

**Diesel (Rust):**
- Compile-time query validation via Rust macros
- Queries referencing deleted columns won't compile
- Zero-cost abstractions philosophy

### Industry Trends

1. **Type-safe query builders are eating ORMs.** jOOQ, Diesel, Jet, Drizzle, sqlc all keep you close to SQL while adding compile-time safety. Full-abstraction ORMs are losing ground.

2. **Schema-first / DB-first is winning.** The database schema is the source of truth. Not Go structs. Not annotations. Not a custom DSL.

3. **Codegen is mainstream.** This was debated in 2020. By 2026, it's the default for serious Go database work.

4. **Nobody's solved migrations + codegen together.** Everyone bolts on Goose, Atlas, or golang-migrate. Integrating these would be a genuine differentiator.

5. **NULL handling in Go is still miserable.** You're choosing between `*string`, `sql.NullString`, or custom types. No tool has cracked this elegantly.

6. **Multi-language codegen is an open frontier.** Prisma's schema language works across TS/Python/Rust (kinda). jOOQ does Java/Kotlin/Scala. Nobody does Go + TypeScript from the same schema.

## What sqlgen Could Be

### Core Philosophy

**Database is the source of truth.** You design your schema with care, migrate it with whatever tool you prefer, and sqlgen generates type-safe code from it. Like jOOQ for Go, but built ground-up for the Go ecosystem.

### Design Principles

1. **Generate from schema, not from a running database.** This is the biggest practical improvement over SQLBoiler and Bob. Parse DDL files or connect to a live DB, your choice. CI pipelines shouldn't need a database running to generate code.

2. **Dialect-specific query builders with shared ergonomics.** Bob got this right. Each dialect gets its own hand-crafted builder. Invalid queries don't compile. But the API patterns should feel consistent across dialects.

3. **Minimal runtime.** Generated code should depend on as little as possible. The less runtime surface area, the easier it is to understand, debug, and extend.

4. **Progressive adoption.** Use the query builder without codegen. Use codegen without the query builder. Use both. Each layer stands alone.

5. **Opinions where they matter, flexibility where they don't.** Automatic timestamps? Option, not mandate. Soft deletes? Same. Type mappings? Configurable. But the generated API shape? Opinionated and consistent.

### Feature Set

#### Must Have (v1)

**Schema introspection:**
- Parse DDL files directly (Postgres, MySQL, SQLite)
- Connect to live database as alternative
- Support for tables, views, materialized views, enums, composite types
- Multi-column primary keys and foreign keys
- Cross-schema relationships

**Code generation:**
- Type-safe model structs with configurable NULL handling
- Full CRUD (insert, update, upsert, delete, select)
- Relationship detection and eager loading (both JOIN and separate-query strategies)
- Column-level whitelisting/blacklisting on operations
- Hooks (before/after for all CRUD operations, with context chaining)
- Enum type generation
- Stale file cleanup (detect and remove generated files for dropped tables)

**Query builder:**
- Dialect-specific, type-safe query construction
- Composable query mods
- Expression builders (no string concatenation for complex conditions)
- Subquery support
- CTE support
- Window functions

**Developer experience:**
- Single binary, no external driver processes
- YAML or TOML config
- Watch mode (regenerate on schema change)
- Clear error messages when schema and generated code drift
- Generated code is readable and debuggable

#### Should Have (v1.x)

**Testing support:**
- Factory generation (like Bob's, inspired by FactoryBot)
- Fixture loading
- Transaction-wrapped test helpers

**Automatic timestamps:**
- `created_at` / `updated_at` auto-population (opt-in)
- Configurable column names

**Soft deletes:**
- `deleted_at` support (opt-in)
- Automatic filtering on queries

**Migration awareness:**
- Integration with Atlas or Goose for drift detection
- "Your schema has changed, here's what's different" warnings

**Raw SQL codegen:**
- sqlc-like layer: write `.sql` files, get type-safe Go functions
- Complements the ORM layer for complex queries

#### Could Have (v2+)

**Multi-language output:**
- TypeScript type generation from the same schema
- Rust struct generation
- Plugin system for adding new target languages

**Schema diffing:**
- Compare two schemas, show what changed
- Generate migration suggestions

**Query analysis:**
- Static analysis of generated queries for performance issues
- Index suggestions based on query patterns

### Architecture Sketch

```
sqlgen/
  cmd/sqlgen/          # CLI entry point
  schema/              # Schema parsing and representation
    parser/            # DDL parsers (pg, mysql, sqlite)
    introspect/        # Live DB introspection
    model.go           # Schema IR (tables, columns, relations, enums)
  gen/                 # Code generation engine
    go/                # Go code generator
      templates/       # Go templates
      types.go         # Type mapping (DB types -> Go types)
    ts/                # TypeScript generator (future)
  dialect/             # Dialect-specific query builders
    postgres/
    mysql/
    sqlite/
  runtime/             # Minimal runtime library (imported by generated code)
    hooks.go
    query.go
    scan.go
  config/              # Configuration parsing
  cli/                 # CLI commands and flags
```

**Key architectural decisions:**

1. **Schema IR (intermediate representation).** Parse DDL or introspect a live DB into a common schema model. All generators work from this IR. This is the foundation for multi-language support later.

2. **Single binary.** No external driver processes. Database-specific code compiles in via build tags or just ships all dialects. The SQLBoiler driver-as-binary approach adds complexity without enough benefit.

3. **Template-based generation with plugin hooks.** Go templates for the bulk of generation (proven pattern), but with clean extension points for custom output.

4. **DDL parsing as first-class.** This is the key differentiator. Parse `CREATE TABLE` statements directly. No running database required. Use a parser like `pganalyze/pg_query_go` for Postgres, `vitess/sqlparser` for MySQL.

### Open Questions

1. **Should we fork SQLBoiler or Bob, or start fresh?**
   - Forking SQLBoiler means inheriting the eager loading mess and the external driver architecture. Lots of cleanup before building new things.
   - Forking Bob means inheriting a cleaner but pre-1.0 codebase with its own rough edges and the `omit.Val` verbosity.
   - Starting fresh means more work upfront but no inherited baggage. Given that the core generation loop is well-understood (both projects prove the pattern), the risk of starting fresh is lower than the risk of inheriting debt.
   - **Recommendation: start fresh, steal ideas liberally from both.** The schema IR and DDL parsing are genuinely new work. The template-based codegen pattern is proven and can be reimplemented cleanly.

2. **How to handle NULL types?**
   - Option A: Generate `*T` (pointer types). Simple, standard, but annoying in templates and JSON.
   - Option B: Generate `sql.NullXxx`. Verbose, nobody likes these.
   - Option C: Generate custom `opt.Val[T]` (like Bob's `omit.Val`). More ergonomic but adds a runtime dependency.
   - Option D: Make it configurable. Let users pick their poison.
   - **Recommendation: Option D, defaulting to a built-in generic `Null[T]` type that handles JSON/SQL scanning cleanly.** Provide escape hatches for `*T` and `sql.NullXxx` via config.

3. **How far to go with the query builder?**
   - Minimal: just generate CRUD methods, let users write raw SQL for complex queries.
   - Medium: composable query mods (like SQLBoiler/Bob) for common patterns.
   - Full: type-safe DSL that can express nearly any valid SQL (like jOOQ/Jet).
   - **Recommendation: start medium, design for full.** The query mod pattern is the right foundation. Build it so the DSL can grow without breaking changes.

4. **DDL parsing scope?**
   - Each database's DDL is its own language. Postgres DDL is particularly complex (extensions, custom types, policies, etc.).
   - **Recommendation: start with Postgres, use `pg_query_go` (wraps Postgres's actual parser). MySQL via `vitess/sqlparser`. SQLite via a lighter parser. Don't try to build a universal DDL parser.**

5. **Name?**
   - `sqlgen` is clean and descriptive. Checks if it's taken on GitHub/pkg.go.dev.
   - Alternatives: `schemagen`, `dbgen`, `forge`, `mint`
