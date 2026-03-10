# sqlgen Gap Analysis: vs SQLBoiler & Bob

Where we stand, and what's worth building next.

## The Short Version

sqlgen covers the core loop plus most of the high-impact features. That's roughly 85% of SQLBoiler's surface area and maybe 60% of Bob's. The remaining gaps are mostly niche query features, testing infrastructure, and additional dialects.

---

## Feature Comparison Matrix

| Feature | SQLBoiler | Bob | sqlgen |
|---------|:---------:|:---:|:------:|
| **Schema Input** | | | |
| Live DB introspection | ✅ (primary) | ✅ | ✅ |
| DDL file parsing | ❌ | ✅ | ✅ (primary) |
| PostgreSQL | ✅ | ✅ | ✅ |
| MySQL | ✅ | ✅ | ❌ |
| SQLite | ✅ (community) | ✅ | ❌ |
| MSSQL | ✅ | ❌ | ❌ |
| CockroachDB | ✅ (community) | ❌ | ❌ |
| **Query Building** | | | |
| SELECT with mods | ✅ | ✅ | ✅ |
| WHERE (AND/OR) | ✅ | ✅ | ✅ (Or, Expr grouping) |
| JOIN (all types) | 4 types | 5 + lateral | ✅ (5 types) |
| GROUP BY / HAVING | ✅ | ✅ | ✅ |
| ORDER BY | ✅ | ✅ (nulls first/last) | ✅ |
| LIMIT / OFFSET | ✅ | ✅ (+ FETCH WITH TIES) | ✅ |
| DISTINCT / DISTINCT ON | ✅ | ✅ | ✅ |
| WhereIn helper | ✅ | ✅ | ✅ |
| CTEs (WITH clause) | ✅ | ✅ (+ recursive) | ✅ (+ recursive) |
| Subqueries | ✅ | ✅ | ❌ |
| UNION / INTERSECT / EXCEPT | ❌ | ✅ | ❌ |
| Window functions | ❌ | ✅ | ❌ |
| Row locking (FOR UPDATE) | ✅ | ✅ (4 lock types) | ✅ (4 types + NOWAIT/SKIP LOCKED) |
| Raw SQL escape hatch | ✅ | ✅ | ✅ (RawSQL) |
| **Mutations** | | | |
| Insert (single row) | ✅ | ✅ | ✅ |
| Insert (multi-row/batch) | ❌ | ✅ | ✅ (InsertAll) |
| Insert from SELECT | ❌ | ✅ | ❌ |
| Update by PK | ✅ | ✅ | ✅ |
| Update (bulk/query-scoped) | ✅ | ✅ | ✅ (UpdateAll) |
| Delete by PK | ✅ | ✅ | ✅ |
| Delete (bulk/query-scoped) | ✅ | ✅ | ✅ (DeleteAll) |
| Upsert | ✅ (dialect-aware) | ✅ (ON CONFLICT) | ✅ (Postgres) |
| Reload from DB | ✅ | ✅ | ✅ |
| Slice UpdateAll/DeleteAll | ✅ | ✅ | ✅ |
| **Relationships** | | | |
| Detection (BelongsTo, Has*, M2M) | ✅ | ✅ | ✅ |
| Eager loading (single level) | ✅ | ✅ | ✅ (LoadRelations) |
| Eager loading (nested) | ✅ (dot notation) | ✅ (nested loaders) | ✅ (dot notation) |
| Preload via LEFT JOIN (to-one) | ❌ | ✅ | ✅ (Preload) |
| Filtered eager loading | ✅ | ✅ | ✅ (mods on Load) |
| Relationship mutation (Set/Add/Remove) | ✅ | ✅ (Attach, Insert) | ❌ |
| Relationship query methods | ❌ | ✅ | ❌ |
| Relationship count loading | ❌ | ✅ | ❌ |
| Polymorphic relationships | ❌ | ✅ | ❌ |
| **Hooks** | | | |
| 9 lifecycle points | ✅ | ✅ | ✅ |
| Hook receives model pointer | ✅ | ✅ | ✅ (typed per-model) |
| Skip via context | ✅ | ✅ | ✅ |
| Disable generation (--no-hooks) | ✅ | ❌ | ✅ |
| **Type System** | | | |
| Nullable: generic wrapper | ✅ (null.String) | ✅ (opt.Val) | ✅ (Null[T]) |
| Nullable: pointer | ✅ | ✅ | ✅ |
| Nullable: database/sql | ✅ | ✅ | ✅ |
| Custom type replacement | ✅ (by type) | ✅ (col/type/nullable) | ✅ (by DB type) |
| Enum generation | ✅ | ✅ | ✅ |
| **Testing** | | | |
| Generated test files | ✅ | ✅ | ✅ |
| Factory/fixture system | ❌ | ✅ (FactoryBot) | ❌ |
| Random data generation | ❌ | ✅ (faker) | ❌ |
| **Developer Experience** | | | |
| Watch mode | ❌ | ❌ | ✅ |
| Stale file cleanup | ⚠️ (--wipe) | ❌ | ✅ (automatic) |
| Debug mode (print SQL) | ✅ | ✅ | ✅ (DebugExecutor) |
| Global DB variant | ✅ (MethodG) | ❌ | ❌ |
| Panic variant | ✅ (MethodP) | ❌ | ❌ |
| Automatic timestamps | ✅ | ❌ | ✅ (configurable) |
| Soft deletes | ✅ | ❌ | ❌ |
| Column whitelist/blacklist | ✅ | ✅ | ✅ |
| Custom templates | ✅ | ✅ | ❌ |
| Struct tag control | ✅ | ✅ | ✅ (configurable) |
| DB error constants | ❌ | ✅ | ❌ |
| Bind to arbitrary struct | ✅ | ❌ | ❌ |
| Prepared statements | ❌ | ✅ | ❌ |
| Query caching | ❌ | ✅ | ❌ |
| Cursor iteration | ❌ | ✅ | ✅ (Each, Cursor) |

---

## Completed

All shipped.

- **OR clauses and Expr() grouping** — `Or("x = ?", 1)`, `Expr(Where(...), Or(...))` for parenthesized groups
- **All 5 JOIN types** — inner, left, right, full, cross
- **DISTINCT / DISTINCT ON** — including Postgres-specific DISTINCT ON
- **Raw SQL escape hatch** — `RawSQL(query, args...)` with Exec/QueryRows/QueryRow
- **Debug executor** — `Debug(exec)` / `DebugTo(exec, writer)` wraps any Executor
- **Eager loading** — generated per-table loaders for all 4 relationship types, dot-notation nesting (`Load("Posts.Tags")`)
- **Bulk UpdateAll / DeleteAll** — query-scoped and slice-scoped
- **Reload** — `model.Reload(ctx, exec)` refreshes by PK
- **WhereIn** — `WhereIn("col", vals...)` helper
- **Generated test files** — opt-in via `output.tests: true`, snapshot/golden testing
- **Typed model hooks** — hooks receive `(ctx, exec, *Model)` so they can inspect/modify the row
- **No-hooks flag** — `output.no_hooks: true` skips hook generation entirely
- **Automatic timestamps** — `created_at`/`updated_at` auto-set on Insert/Update, configurable column names
- **Column whitelist/blacklist** — `Whitelist("email", "name")` / `Blacklist("id")` for Insert/Update/Upsert
- **Filtered eager loading** — `Load("Posts", Where(...))` passes mods through to loader queries
- **Preload via LEFT JOIN** — `Preload(PostPreloadUser)` folds to-one relationships into the parent SELECT
- **CTEs (WITH clause)** — `WithCTE(name, query)` and `WithRecursiveCTE(name, query)` for hierarchical queries
- **Row locking** — `ForUpdate()`, `ForShare()`, `ForNoKeyUpdate()`, `ForKeyShare()` with `Nowait()` and `SkipLocked()`
- **Cursor/streaming iteration** — `EachX()` callback and `XCursor()` for memory-efficient row processing
- **Struct tag customization** — configurable tags and casing via `tags` config map
- **Batch insert** — `InsertAll()` on slice types for multi-row INSERT with RETURNING

---

## Remaining Gaps (Priority Order)

### Medium Impact

**1. Custom type replacement by column name**
Replacements only match by DB type. Matching by column name/nullability is useful for: "all `metadata` columns → `json.RawMessage`."

**2. Factory/fixture system**
Generated test files exist, but no factory system for generating test data. Bob's FactoryBot-inspired factories are genuinely useful for integration tests.

**3. Relationship mutations (Set/Add/Remove)**
Can't programmatically add/remove related records through the relationship API. Both competitors have this.

### Lower Impact

**4. UNION / INTERSECT / EXCEPT** — Bob has these, SQLBoiler doesn't.
**5. Soft deletes** — SQLBoiler has first-class support. Doable with hooks.
**6. Custom templates** — Both competitors let users override templates.
**7. DB error matching** — Bob generates typed constraint error matchers.
**8. Prepared statement caching** — Performance optimization. Bob only.

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
- Custom template support
- Type replacement by column name/nullability

**Phase 16: Testing Support** (medium impact, high effort)
- Factory system with random data generation
- Typed DB error matchers

**Phase 17: More Dialects** (high impact, high effort)
- MySQL driver
- SQLite driver
