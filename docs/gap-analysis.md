# sqlgen Gap Analysis: vs SQLBoiler & Bob

Where we stand after Phases 1-9, and what's worth building next.

## The Short Version

sqlgen covers the core loop plus most of the high-impact features: DDL parsing, live DB introspection, typed models, CRUD, where clauses (with OR and grouping), enums, hooks, relationship detection, eager loading with dot-notation nesting, bulk operations, reload, raw SQL, and debug mode. That's roughly 80% of SQLBoiler's surface area and maybe 50% of Bob's.

The remaining gaps are mostly about DX polish, testing infrastructure, and advanced query features.

---

## Feature Comparison Matrix

| Feature | SQLBoiler | Bob | sqlgen |
|---------|-----------|-----|--------|
| **Schema Input** | | | |
| Live DB introspection | Yes (primary method) | Yes | Yes |
| DDL file parsing | No | Yes (SQL file driver) | Yes (primary method) |
| PostgreSQL | Yes | Yes | Yes |
| MySQL | Yes | Yes | No |
| SQLite | Yes (community) | Yes | No |
| MSSQL | Yes | No | No |
| CockroachDB | Yes (community) | No | No |
| **Query Building** | | | |
| SELECT with mods | Yes | Yes | Yes |
| WHERE (AND/OR) | Yes (And, Or, Or2) | Yes | **Yes** (Or, Expr grouping) |
| JOIN (all types) | 4 types | 5 types + lateral | **Yes** (inner, left, right, full, cross) |
| GROUP BY / HAVING | Yes | Yes | Yes |
| ORDER BY | Yes | Yes (with nulls first/last) | Yes |
| LIMIT / OFFSET | Yes | Yes (+ FETCH WITH TIES) | Yes |
| DISTINCT / DISTINCT ON | Yes | Yes (Postgres DISTINCT ON) | **Yes** |
| WhereIn helper | Yes | Yes | **Yes** |
| CTEs (WITH clause) | Yes | Yes (+ recursive) | **Yes** (+ recursive) |
| Subqueries | Yes | Yes | No |
| UNION / INTERSECT / EXCEPT | No | Yes | No |
| Window functions | No | Yes | No |
| Row locking (FOR UPDATE) | Yes | Yes (4 lock types) | **Yes** (4 types + NOWAIT/SKIP LOCKED) |
| Raw SQL escape hatch | Yes (qm.SQL, queries.Raw) | Yes (Raw, RawQuery) | **Yes** (RawSQL) |
| **Mutations** | | | |
| Insert (single row) | Yes | Yes | Yes |
| Insert (multi-row/batch) | No | Yes (Values, Rows) | No |
| Insert from SELECT | No | Yes | No |
| Update by PK | Yes | Yes | Yes |
| Update (bulk/query-scoped) | Yes (UpdateAll) | Yes (UpdateAll) | **Yes** (UpdateAll) |
| Delete by PK | Yes | Yes | Yes |
| Delete (bulk/query-scoped) | Yes (DeleteAll) | Yes (DeleteAll) | **Yes** (DeleteAll) |
| Upsert | Yes (dialect-aware) | Yes (ON CONFLICT) | Yes (Postgres only) |
| Reload from DB | Yes | Yes | **Yes** |
| Slice UpdateAll/DeleteAll | Yes | Yes | **Yes** (single-column PK) |
| **Relationships** | | | |
| Detection (BelongsTo, Has*, M2M) | Yes | Yes | Yes |
| Eager loading (single level) | Yes (qm.Load) | Yes (Preload, ThenLoad) | **Yes** (LoadRelations) |
| Eager loading (nested/recursive) | Yes (dot notation) | Yes (nested loaders) | **Yes** (dot notation) |
| Preload via LEFT JOIN (to-one) | No | Yes (Preload) | **Yes** (Preload) |
| Filtered eager loading | Yes (mods on Load) | Yes (mods on ThenLoad) | **Yes** (mods on Load) |
| Relationship mutation (Set/Add/Remove) | Yes | Yes (Attach, Insert) | No |
| Relationship query methods | No | Yes (returns TableQuery) | No |
| Relationship count loading | No | Yes (PreloadCount, ThenLoadCount) | No |
| Polymorphic relationships | No | Yes (from_where/to_where) | No |
| **Hooks** | | | |
| 9 lifecycle points | Yes | Yes | Yes |
| Hook receives model pointer | Yes | Yes | **Yes** (typed per-model) |
| Skip via context | Yes | Yes (model + query separately) | Yes |
| Disable generation (--no-hooks) | Yes | No | **Yes** (output.no_hooks) |
| **Type System** | | | |
| Nullable: generic wrapper | null.String (v8 lib) | opt.Val (aarondl/opt) | Null[T] (built-in generic) |
| Nullable: pointer | Via type replacement | Via type replacement | Yes (config option) |
| Nullable: database/sql | Via type replacement | Via type replacement | Yes (config option) |
| Custom type replacement | Yes (by type match) | Yes (by column/type/nullable) | Yes (by DB type name) |
| Enum generation | Yes (--add-enum-types) | Yes | Yes |
| **Testing** | | | |
| Generated test files | Yes | Yes | **Yes** (output.tests) |
| Factory/fixture system | No | Yes (FactoryBot-inspired) | No |
| Random data generation | No | Yes (jaswdr/faker) | No |
| **Developer Experience** | | | |
| Watch mode | No | No | Yes |
| Stale file cleanup | No (--wipe flag) | No | Yes (automatic) |
| Debug mode (print SQL) | Yes (boil.DebugMode) | Yes (bob.Debug) | **Yes** (DebugExecutor) |
| Global DB variant (no exec param) | Yes (MethodG) | No | No |
| Panic variant | Yes (MethodP) | No | No |
| Automatic timestamps | Yes (created_at/updated_at) | No | **Yes** (configurable columns) |
| Soft deletes | Yes (--add-soft-deletes) | No | No |
| Column whitelist/blacklist on mutations | Yes (Infer/Whitelist/Blacklist/Greylist) | Yes (Only/Except) | **Yes** (Whitelist/Blacklist) |
| Custom templates | Yes | Yes | No |
| Struct tag control | Yes (casing, extra tags) | Yes (casing, extra tags) | No (hardcoded json+db) |
| DB error constants | No | Yes (dberrors plugin) | No |
| Bind to arbitrary struct | Yes (Bind finisher) | No | No |
| Prepared statement support | No | Yes (Prepare, PrepareQuery) | No |
| Query caching | No | Yes (bob.Cache) | No |
| Cursor iteration | No | Yes (Cursor, Each) | **Yes** (Each, Cursor) |

---

## Completed (Phases 7-14)

These were the highest-impact gaps. All shipped.

- **OR clauses and Expr() grouping** - `Or("x = ?", 1)`, `Expr(Where(...), Or(...))` for parenthesized groups
- **All 5 JOIN types** - inner, left, right, full, cross
- **DISTINCT / DISTINCT ON** - including Postgres-specific DISTINCT ON
- **Raw SQL escape hatch** - `RawSQL(query, args...)` with Exec/QueryRows/QueryRow
- **Debug executor** - `Debug(exec)` / `DebugTo(exec, writer)` wraps any Executor
- **Eager loading** - generated per-table loaders for all 4 relationship types, dot-notation nesting (`Load("Posts.Tags")`)
- **Bulk UpdateAll / DeleteAll** - query-scoped and slice-scoped
- **Reload** - `model.Reload(ctx, exec)` refreshes by PK
- **WhereIn** - `WhereIn("col", vals...)` helper
- **Generated test files** - opt-in via `output.tests: true`, snapshot/golden testing
- **Typed model hooks** - hooks receive `(ctx, exec, *Model)` so they can inspect/modify the row
- **No-hooks flag** - `output.no_hooks: true` skips hook generation entirely
- **Automatic timestamps** - `created_at`/`updated_at` auto-set on Insert/Update, configurable column names
- **Column whitelist/blacklist** - `Whitelist("email", "name")` / `Blacklist("id")` for Insert/Update/Upsert
- **Filtered eager loading** - `Load("Posts", Where(...))` passes mods through to loader queries
- **Preload via LEFT JOIN** - `Preload(PostPreloadUser)` folds to-one relationships into the parent SELECT
- **CTEs (WITH clause)** - `WithCTE(name, query)` and `WithRecursiveCTE(name, query)` for hierarchical queries
- **Row locking** - `ForUpdate()`, `ForShare()`, `ForNoKeyUpdate()`, `ForKeyShare()` with `Nowait()` and `SkipLocked()`
- **Cursor/streaming iteration** - `EachX()` callback and `XCursor()` for memory-efficient row processing

---

## Remaining Gaps (Priority Order)

### Medium Impact

**1. Struct tag customization**
Hardcoded to `json:"snake" db:"name"`. Users want `yaml`, `toml`, camelCase JSON, or custom tags.

**2. Custom type replacement by column name**
Replacements only match by DB type. Matching by column name/nullability is useful for: "all `metadata` columns → `json.RawMessage`."

**3. Factory/fixture system**
Generated test files exist, but no factory system for generating test data. Bob's FactoryBot-inspired factories are genuinely useful for integration tests.

**4. Relationship mutations (Set/Add/Remove)**
Can't programmatically add/remove related records through the relationship API. Both competitors have this.

### Lower Impact

**5. UNION / INTERSECT / EXCEPT** - Bob has these, SQLBoiler doesn't.
**6. Soft deletes** - SQLBoiler has first-class support. Doable with hooks.
**7. Custom templates** - Both competitors let users override templates.
**8. DB error matching** - Bob generates typed constraint error matchers.
**9. Prepared statement caching** - Performance optimization. Bob only.
**10. Batch insert** - Multi-row INSERT. Bob only.

---

## What sqlgen Does That They Don't

- **DDL parsing as primary input.** SQLBoiler requires a running database. sqlgen treats DDL files as first-class, so you can generate code in CI without a database.
- **Watch mode.** Neither SQLBoiler nor Bob has file watching.
- **Automatic stale file cleanup.** Drop a table, regenerate, old files disappear. SQLBoiler has `--wipe` (nukes everything). Bob has nothing.
- **Built-in generic Null[T].** No external dependency. SQLBoiler uses `volatiletech/null`, Bob uses `aarondl/opt`.
- **Env var expansion in DSN.** Neither competitor does it.

---

## Suggested Roadmap

**Phase 15: Developer Experience** (medium impact, medium effort)
- Struct tag customization (casing, extra tags)
- Custom template support
- Type replacement by column name/nullability

**Phase 16: Testing Support** (medium impact, high effort)
- Factory system with random data generation
- Typed DB error matchers

**Phase 17: More Dialects** (high impact, high effort)
- MySQL driver
- SQLite driver
