# sqlgen Gap Analysis: vs SQLBoiler & Bob

Where we stand, and what's worth building next.

## The Short Version

sqlgen covers the core loop plus nearly all high-impact features. That's roughly **95% of SQLBoiler's surface area** and maybe **80% of Bob's**. The remaining gaps are mostly niche features, testing infrastructure, and additional dialects.

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
| Subqueries | ✅ | ✅ | ✅ (WhereSubquery, WhereExists, FromSubquery) |
| UNION / INTERSECT / EXCEPT | ❌ | ✅ | ✅ (+ ALL variants) |
| Window functions | ❌ | ✅ | ✅ (WindowDef, SelectWithWindow) |
| Row locking (FOR UPDATE) | ✅ | ✅ (4 lock types) | ✅ (4 types + NOWAIT/SKIP LOCKED) |
| Raw SQL escape hatch | ✅ | ✅ | ✅ (RawSQL) |
| **Mutations** | | | |
| Insert (single row) | ✅ | ✅ | ✅ |
| Insert (multi-row/batch) | ❌ | ✅ | ✅ (InsertAll) |
| Insert from SELECT | ❌ | ✅ | ✅ (BuildInsertSelect) |
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
| Relationship mutation (Set/Add/Remove) | ✅ | ✅ (Attach, Insert) | ✅ (Set/Add/Remove) |
| Relationship count loading | ❌ | ✅ | ✅ (LoadCountRelations) |
| Polymorphic relationships | ❌ | ✅ | ✅ (config-driven) |
| **Hooks** | | | |
| 9 lifecycle points | ✅ | ✅ | ✅ |
| Hook receives model pointer | ✅ | ✅ | ✅ (typed per-model) |
| Skip via context | ✅ | ✅ | ✅ |
| Disable generation (--no-hooks) | ✅ | ❌ | ✅ |
| **Type System** | | | |
| Nullable: generic wrapper | ✅ (null.String) | ✅ (opt.Val) | ✅ (Null[T]) |
| Nullable: pointer | ✅ | ✅ | ✅ |
| Nullable: database/sql | ✅ | ✅ | ✅ |
| Custom type replacement | ✅ (by type) | ✅ (col/type/nullable) | ✅ (by DB type + column name) |
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
| Custom templates | ✅ | ✅ | ✅ (directory overlay) |
| Struct tag control | ✅ | ✅ | ✅ (configurable) |
| DB error constants | ❌ | ✅ | ❌ |
| Bind to arbitrary struct | ✅ | ❌ | ✅ (runtime.Bind) |
| Prepared statements | ❌ | ✅ | ❌ |
| Query caching | ❌ | ✅ | ❌ |
| Cursor iteration | ❌ | ✅ | ✅ (Each, Cursor) |

---

## Reverse Gap: sqlgen-Only Features

Things sqlgen has that neither SQLBoiler nor Bob do.

- **DDL parsing as primary input.** SQLBoiler requires a running database. sqlgen treats DDL files as first-class, so you can generate code in CI without a database.
- **Watch mode.** Neither SQLBoiler nor Bob has file watching. `sqlgen watch` regenerates on save.
- **Automatic stale file cleanup.** Drop a table, regenerate, old files disappear. SQLBoiler has `--wipe` (nukes everything). Bob has nothing.
- **Built-in generic Null[T].** Zero external dependencies. SQLBoiler uses `volatiletech/null`, Bob uses `aarondl/opt`.
- **Env var expansion in DSN.** `${DATABASE_URL}` works in config. Neither competitor does it.
- **Batch insert with InsertAll.** SQLBoiler has no multi-row insert. sqlgen generates `InsertAll()` on slice types with RETURNING scan-back.
- **UNION/INTERSECT/EXCEPT.** SQLBoiler doesn't have these at all. sqlgen supports all 6 variants.
- **Window functions.** SQLBoiler doesn't have window function support. sqlgen has `WindowDef` + `SelectWithWindow`.
- **Insert from SELECT.** SQLBoiler doesn't support `INSERT INTO ... SELECT`. sqlgen has `BuildInsertSelect`.
- **Relationship count loading.** SQLBoiler can't load counts. sqlgen generates `LoadCountRelations` with `COUNT(*) GROUP BY`.
- **Polymorphic relationships.** SQLBoiler doesn't support polymorphic. sqlgen has config-driven polymorphic with type+id columns.
- **No-hooks flag.** Bob can't disable hook generation. sqlgen has `output.no_hooks: true`.
- **Configurable automatic timestamps.** Bob doesn't have automatic timestamps at all.

---

## Remaining Gaps

### Medium Impact

**1. Factory/fixture system**
Generated test files exist, but no factory system for generating test data. Bob's FactoryBot-inspired factories are genuinely useful for integration tests.

### Lower Impact

**2. Soft deletes** — SQLBoiler has first-class support. Doable with hooks.
**3. DB error matching** — Bob generates typed constraint error matchers.
**4. Prepared statement caching** — Performance optimization. Bob only.

---

## Suggested Roadmap

**Phase 15: Testing Support** (medium impact, high effort)
- Factory system with random data generation
- Typed DB error matchers

**Phase 16: More Dialects** (high impact, high effort)
- MySQL driver
- SQLite driver
