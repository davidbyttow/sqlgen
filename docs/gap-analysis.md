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
| CTEs (WITH clause) | Yes | Yes (+ recursive) | No |
| Subqueries | Yes | Yes | No |
| UNION / INTERSECT / EXCEPT | No | Yes | No |
| Window functions | No | Yes | No |
| Row locking (FOR UPDATE) | Yes | Yes (4 lock types) | No |
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
| Filtered eager loading | Yes (mods on Load) | Yes (mods on ThenLoad) | No |
| Relationship mutation (Set/Add/Remove) | Yes | Yes (Attach, Insert) | No |
| Relationship query methods | No | Yes (returns TableQuery) | No |
| Relationship count loading | No | Yes (PreloadCount, ThenLoadCount) | No |
| Polymorphic relationships | No | Yes (from_where/to_where) | No |
| **Hooks** | | | |
| 9 lifecycle points | Yes | Yes | Yes |
| Hook receives model pointer | Yes | Yes | No (context only) |
| Skip via context | Yes | Yes (model + query separately) | Yes |
| Disable generation (--no-hooks) | Yes | No | No |
| **Type System** | | | |
| Nullable: generic wrapper | null.String (v8 lib) | opt.Val (aarondl/opt) | Null[T] (built-in generic) |
| Nullable: pointer | Via type replacement | Via type replacement | Yes (config option) |
| Nullable: database/sql | Via type replacement | Via type replacement | Yes (config option) |
| Custom type replacement | Yes (by type match) | Yes (by column/type/nullable) | Yes (by DB type name) |
| Enum generation | Yes (--add-enum-types) | Yes | Yes |
| **Testing** | | | |
| Generated test files | Yes | Yes | No |
| Factory/fixture system | No | Yes (FactoryBot-inspired) | No |
| Random data generation | No | Yes (jaswdr/faker) | No |
| **Developer Experience** | | | |
| Watch mode | No | No | Yes |
| Stale file cleanup | No (--wipe flag) | No | Yes (automatic) |
| Debug mode (print SQL) | Yes (boil.DebugMode) | Yes (bob.Debug) | **Yes** (DebugExecutor) |
| Global DB variant (no exec param) | Yes (MethodG) | No | No |
| Panic variant | Yes (MethodP) | No | No |
| Automatic timestamps | Yes (created_at/updated_at) | No | No |
| Soft deletes | Yes (--add-soft-deletes) | No | No |
| Column whitelist/blacklist on mutations | Yes (Infer/Whitelist/Blacklist/Greylist) | Yes (Only/Except) | No |
| Custom templates | Yes | Yes | No |
| Struct tag control | Yes (casing, extra tags) | Yes (casing, extra tags) | No (hardcoded json+db) |
| DB error constants | No | Yes (dberrors plugin) | No |
| Bind to arbitrary struct | Yes (Bind finisher) | No | No |
| Prepared statement support | No | Yes (Prepare, PrepareQuery) | No |
| Query caching | No | Yes (bob.Cache) | No |
| Cursor iteration | No | Yes (Cursor, Each) | No |

---

## Completed (Phases 7-9)

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

---

## Remaining Gaps (Priority Order)

### High Impact

**1. Hook receives model**
Our hooks get `(ctx) -> (ctx, error)`. Both competitors pass the model pointer. Without it, hooks can't inspect or modify the row. This limits hooks to logging/tracing.

**2. Column whitelist/blacklist on mutations**
Can't do partial updates ("update only name and email") or partial inserts ("insert everything except id"). SQLBoiler's `Whitelist`/`Blacklist`/`Infer` and Bob's `Only`/`Except` patterns handle this.

**3. Filtered eager loading**
Our `Load("Posts")` works, but can't filter: `Load("Posts", Where("status = ?", "published"))`. The `EagerLoadRequest.Mods` field exists but the generated loaders don't pass them through yet.

**4. Automatic timestamps**
SQLBoiler auto-manages `created_at`/`updated_at`. Every project needs this; without it everyone writes the same hook.

### Medium Impact

**5. Struct tag customization**
Hardcoded to `json:"snake" db:"name"`. Users want `yaml`, `toml`, camelCase JSON, or custom tags.

**6. Custom type replacement by column name**
Replacements only match by DB type. Matching by column name/nullability is useful for: "all `metadata` columns → `json.RawMessage`."

**7. Testing support**
No generated tests, no factory system. Bob's FactoryBot-inspired factories are genuinely useful for integration tests.

**8. Cursor/streaming iteration**
For large result sets. Bob has `Cursor()` and `Each()`.

**9. Relationship mutations (Set/Add/Remove)**
Can't programmatically add/remove related records through the relationship API. Both competitors have this.

### Lower Impact

**10. CTEs (WITH clause)** - Useful for recursive queries.
**11. Row locking (FOR UPDATE/SHARE)** - Needed for transactional workflows.
**12. UNION / INTERSECT / EXCEPT** - Bob has these, SQLBoiler doesn't.
**13. Soft deletes** - SQLBoiler has first-class support. Doable with hooks.
**14. Custom templates** - Both competitors let users override templates.
**15. DB error matching** - Bob generates typed constraint error matchers.
**16. Prepared statement caching** - Performance optimization. Bob only.
**17. Batch insert** - Multi-row INSERT. Bob only.

---

## What sqlgen Does That They Don't

- **DDL parsing as primary input.** SQLBoiler requires a running database. sqlgen treats DDL files as first-class, so you can generate code in CI without a database.
- **Watch mode.** Neither SQLBoiler nor Bob has file watching.
- **Automatic stale file cleanup.** Drop a table, regenerate, old files disappear. SQLBoiler has `--wipe` (nukes everything). Bob has nothing.
- **Built-in generic Null[T].** No external dependency. SQLBoiler uses `volatiletech/null`, Bob uses `aarondl/opt`.
- **Env var expansion in DSN.** Neither competitor does it.

---

## Suggested Roadmap

**Phase 10: Hooks v2 + Timestamps** (high impact, low effort)
- Pass model pointer to hooks: `func(ctx, exec, *Model) (ctx, error)`
- Automatic `created_at`/`updated_at` management
- `--no-hooks` generation flag

**Phase 11: Mutation Control + Filtered Loading** (high impact, medium effort)
- Column whitelist/blacklist for Insert/Update
- Wire `EagerLoadRequest.Mods` through generated loaders

**Phase 12: Developer Experience** (medium impact, medium effort)
- Struct tag customization (casing, extra tags)
- Custom template support
- Type replacement by column name/nullability

**Phase 13: Testing Support** (medium impact, high effort)
- Generated test files
- Factory system with random data generation
- Typed DB error matchers

**Phase 14: Advanced Query Features** (medium impact, medium effort)
- CTEs (WITH clause)
- Row locking (FOR UPDATE/SHARE)
- Cursor/streaming iteration

**Phase 15: More Dialects** (high impact, high effort)
- MySQL driver
- SQLite driver
