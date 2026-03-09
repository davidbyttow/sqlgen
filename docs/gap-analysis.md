# sqlgen Gap Analysis: vs SQLBoiler & Bob

Where we stand after v1, and what's worth building next.

## The Short Version

sqlgen v1 covers the core loop: parse DDL (or introspect a live DB), generate typed models, CRUD, where clauses, enums, hooks, and relationship detection. That's roughly 60% of SQLBoiler's surface area and maybe 30% of Bob's. The gaps fall into a few buckets: query power, relationship operations, testing support, and developer ergonomics.

---

## Feature Comparison Matrix

| Feature | SQLBoiler | Bob | sqlgen v1 |
|---------|-----------|-----|-----------|
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
| WHERE (AND/OR) | Yes (And, Or, Or2) | Yes | Partial (AND only) |
| JOIN (all types) | 4 types | 5 types + lateral | 2 types (inner, left) |
| GROUP BY / HAVING | Yes | Yes | Yes |
| ORDER BY | Yes | Yes (with nulls first/last) | Yes |
| LIMIT / OFFSET | Yes | Yes (+ FETCH WITH TIES) | Yes |
| DISTINCT / DISTINCT ON | Yes | Yes (Postgres DISTINCT ON) | No |
| CTEs (WITH clause) | Yes | Yes (+ recursive) | No |
| Subqueries | Yes | Yes | No |
| UNION / INTERSECT / EXCEPT | No | Yes | No |
| Window functions | No | Yes | No |
| Row locking (FOR UPDATE) | Yes | Yes (4 lock types) | No |
| Raw SQL escape hatch | Yes (qm.SQL, queries.Raw) | Yes (Raw, RawQuery) | No |
| **Mutations** | | | |
| Insert (single row) | Yes | Yes | Yes |
| Insert (multi-row/batch) | No | Yes (Values, Rows) | No |
| Insert from SELECT | No | Yes | No |
| Update by PK | Yes | Yes | Yes |
| Update (bulk/query-scoped) | Yes (UpdateAll) | Yes (UpdateAll) | No |
| Delete by PK | Yes | Yes | Yes |
| Delete (bulk/query-scoped) | Yes (DeleteAll) | Yes (DeleteAll) | No |
| Upsert | Yes (dialect-aware) | Yes (ON CONFLICT) | Yes (Postgres only) |
| Reload from DB | Yes | Yes | No |
| **Relationships** | | | |
| Detection (BelongsTo, Has*, M2M) | Yes | Yes | Yes |
| Eager loading (single level) | Yes (qm.Load) | Yes (Preload, ThenLoad) | No (helpers exist, not generated) |
| Eager loading (nested/recursive) | Yes (dot notation) | Yes (nested loaders) | No |
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
| Debug mode (print SQL) | Yes (boil.DebugMode) | Yes (bob.Debug) | No |
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

## Priority Gaps (High Impact)

These are the features whose absence users will feel immediately when trying to use sqlgen for real work.

### 1. OR clauses and grouped conditions
Right now every Where mod is ANDed. Can't express `WHERE (status = 'active' OR status = 'pending') AND org_id = ?`. SQLBoiler has `Or()`, `Or2()`, `Expr()` for grouping. Bob has `Or()`, `And()`, `Group()` expression builders.

### 2. Eager loading (generated, not just helpers)
The runtime has `LoadMany`, `LoadOne`, `LoadManyToMany`, but none of the generated code calls them. Users can't do `Users(Load("Posts"))` or anything equivalent. Both SQLBoiler and Bob generate this automatically. This is probably the single biggest missing piece.

### 3. Bulk operations (UpdateAll, DeleteAll)
Can't do "update all posts where status = draft" or "delete all expired sessions" without writing raw SQL. Both competitors generate query-scoped bulk mutations.

### 4. Raw SQL escape hatch
No way to drop down to raw SQL when the query builder doesn't cover your case. SQLBoiler has `queries.Raw()` and `qm.SQL()`. Bob has `RawQuery()` and `Raw()`.

### 5. Reload
No `user.Reload(ctx, db)` to refresh a model from the database after external changes. Both SQLBoiler and Bob have this.

### 6. Debug mode
No way to print the SQL being executed. Both competitors have this. Essential during development.

### 7. Hook receives model
Our hooks get `(ctx) -> (ctx, error)`. SQLBoiler passes `(ctx, exec, *Model)`. Bob passes `(ctx, exec, model)`. Without the model, hooks can't inspect or modify the row being mutated, which limits their usefulness to logging/tracing.

---

## Medium Priority Gaps

Things that matter but aren't blockers for getting started.

### 8. Additional JOIN types
Only inner and left join. Missing: right join, full outer join, cross join. SQLBoiler has 4, Bob has 5 plus lateral joins.

### 9. DISTINCT / DISTINCT ON
No distinct support at all. Postgres's DISTINCT ON is particularly useful.

### 10. Column selection on mutations
Can't say "insert only these columns" or "update everything except these columns." SQLBoiler's `Whitelist`/`Blacklist`/`Infer` pattern is ergonomic for partial updates.

### 11. Automatic timestamps
SQLBoiler auto-manages `created_at` and `updated_at`. Common enough that not having it means every user writes the same hook.

### 12. Struct tag customization
Hardcoded to `json:"snake" db:"name"`. Users might want `yaml`, `toml`, `gorm`, camelCase JSON, or custom tags.

### 13. Custom type replacement by column name
Our replacements only match by DB type name. SQLBoiler and Bob can match by column name, nullability, and other attributes. Useful for: "all columns named `metadata` should be `json.RawMessage`."

### 14. Testing support
No generated tests, no factory system. Bob's FactoryBot-inspired factories with faker integration are genuinely useful. SQLBoiler generates test files that run against a real DB.

### 15. Cursor/streaming iteration
For large result sets, loading everything into memory isn't great. Bob has `Cursor()` and `Each()` for streaming.

---

## Lower Priority (Nice to Have)

### 16. CTEs (WITH clause)
Both SQLBoiler and Bob support them. Useful for recursive queries and complex reporting.

### 17. UNION / INTERSECT / EXCEPT
Bob supports all set operations. SQLBoiler doesn't.

### 18. Row locking (FOR UPDATE/SHARE)
Both competitors support this. Matters for transactional workflows.

### 19. Window functions
Only Bob has these. Probably not needed in generated code, more of a raw SQL thing.

### 20. Soft deletes
SQLBoiler has first-class support. Bob doesn't. Useful but opinionated; can be done with hooks.

### 21. Custom templates
Both competitors let users override/extend templates. Good for shops with specific conventions.

### 22. Prepared statement support
Only Bob has this. Performance optimization for hot paths.

### 23. Query caching
Only Bob has this. Same category.

### 24. DB error matching
Bob generates typed error matchers for constraint violations. Nice DX for handling unique conflicts, FK violations, etc.

---

## What sqlgen Does That They Don't

A few things we got right that the competitors missed or skipped:

- **DDL parsing as primary input.** SQLBoiler requires a running database. Bob supports SQL files but it's a secondary driver. sqlgen treats DDL files as a first-class citizen, which means you can generate code in CI without a database.
- **Watch mode.** Neither SQLBoiler nor Bob has file watching. We do.
- **Automatic stale file cleanup.** Drop a table from your DDL, regenerate, and the old files disappear. SQLBoiler has `--wipe` (nuke everything). Bob has nothing.
- **Built-in generic Null[T].** No external dependency. SQLBoiler uses `volatiletech/null`, Bob uses `aarondl/opt`. Our `Null[T]` is simpler and ships with the runtime.
- **Env var expansion in DSN.** Small thing, but neither competitor does it.

---

## Suggested Roadmap

Based on impact and effort, here's a rough order for what to build next:

**Phase 7: Query Power** (high impact, medium effort)
- OR clauses and expression grouping
- Raw SQL escape hatch
- DISTINCT / DISTINCT ON
- Right join, full outer join, cross join
- Debug mode (print SQL + args)

**Phase 8: Relationship Loading** (high impact, high effort)
- Generated eager loading methods per relationship
- `Load("Relationship")` query mod
- Nested/dot-notation loading
- Filtered eager loading (mods on the loaded relationship)

**Phase 9: Bulk Operations + Reload** (high impact, medium effort)
- Query-scoped `UpdateAll(ctx, exec, set)` and `DeleteAll(ctx, exec)`
- `Reload(ctx, exec)` on model instances
- Column whitelist/blacklist for mutations

**Phase 10: Hooks v2 + Timestamps** (medium impact, low effort)
- Pass model pointer to hooks: `func(ctx, exec, *Model) (ctx, error)`
- Automatic `created_at`/`updated_at` management
- `--no-hooks` generation flag

**Phase 11: Developer Experience** (medium impact, medium effort)
- Struct tag customization (casing, extra tags)
- Custom template support
- Type replacement by column name/nullability
- Debug executor wrapper

**Phase 12: Testing Support** (medium impact, high effort)
- Generated test files
- Factory system with random data generation
- Typed DB error matchers

**Phase 13: More Dialects** (high impact, high effort)
- MySQL driver
- SQLite driver
