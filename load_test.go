package sqlgen

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLoadManyWithMods(t *testing.T) {
	load := Load("Posts", Where(`"status" = ?`, "published"), Limit(5))
	if load.Name != "Posts" {
		t.Errorf("Name = %q", load.Name)
	}
	if len(load.Mods) != 2 {
		t.Errorf("Mods = %d, want 2", len(load.Mods))
	}
}

func TestLoadNestedMods(t *testing.T) {
	// Mods on dot-notation go to the leaf.
	load := Load("Posts.Tags", Where(`"active" = ?`, true))
	if load.Name != "Posts" {
		t.Errorf("Name = %q", load.Name)
	}
	if len(load.Mods) != 0 {
		t.Errorf("root Mods = %d, want 0", len(load.Mods))
	}
	if len(load.Nested) != 1 {
		t.Fatalf("Nested = %d, want 1", len(load.Nested))
	}
	if len(load.Nested[0].Mods) != 1 {
		t.Errorf("leaf Mods = %d, want 1", len(load.Nested[0].Mods))
	}
}

func TestBuildInClause(t *testing.T) {
	d := PostgresDialect{}
	clause := buildInClause(d, "id", 3)
	if !strings.Contains(clause, `"id" IN (?, ?, ?)`) {
		t.Errorf("clause = %q", clause)
	}
}

func TestBuildInClauseWithPrefix(t *testing.T) {
	d := PostgresDialect{}
	clause := buildInClauseWithPrefix(d, "__jt", "post_id", 2)
	if !strings.Contains(clause, `__jt."post_id" IN (?, ?)`) {
		t.Errorf("clause = %q", clause)
	}
}

// --- New tests ---

func TestLoadThreeLevelDotNotation(t *testing.T) {
	// "Author.Posts.Tags" should produce 3 nested levels.
	load := Load("Author.Posts.Tags")

	if load.Name != "Author" {
		t.Errorf("root Name = %q, want Author", load.Name)
	}
	if len(load.Mods) != 0 {
		t.Errorf("root Mods = %d, want 0", len(load.Mods))
	}
	if len(load.Nested) != 1 {
		t.Fatalf("root Nested = %d, want 1", len(load.Nested))
	}

	mid := load.Nested[0]
	if mid.Name != "Posts" {
		t.Errorf("mid Name = %q, want Posts", mid.Name)
	}
	if len(mid.Mods) != 0 {
		t.Errorf("mid Mods = %d, want 0", len(mid.Mods))
	}
	if len(mid.Nested) != 1 {
		t.Fatalf("mid Nested = %d, want 1", len(mid.Nested))
	}

	leaf := mid.Nested[0]
	if leaf.Name != "Tags" {
		t.Errorf("leaf Name = %q, want Tags", leaf.Name)
	}
	if len(leaf.Nested) != 0 {
		t.Errorf("leaf Nested = %d, want 0", len(leaf.Nested))
	}
}

func TestLoadThreeLevelModsOnLeaf(t *testing.T) {
	// Mods should only appear on the deepest nested request.
	load := Load("Author.Posts.Tags", Where(`"active" = ?`, true), Limit(10))

	if len(load.Mods) != 0 {
		t.Errorf("root Mods = %d, want 0", len(load.Mods))
	}

	mid := load.Nested[0]
	if len(mid.Mods) != 0 {
		t.Errorf("mid Mods = %d, want 0", len(mid.Mods))
	}

	leaf := mid.Nested[0]
	if len(leaf.Mods) != 2 {
		t.Errorf("leaf Mods = %d, want 2", len(leaf.Mods))
	}
}

func TestLoadFourLevelDotNotation(t *testing.T) {
	load := Load("A.B.C.D")
	if load.Name != "A" {
		t.Errorf("level 0 = %q", load.Name)
	}
	l1 := load.Nested[0]
	if l1.Name != "B" {
		t.Errorf("level 1 = %q", l1.Name)
	}
	l2 := l1.Nested[0]
	if l2.Name != "C" {
		t.Errorf("level 2 = %q", l2.Name)
	}
	l3 := l2.Nested[0]
	if l3.Name != "D" {
		t.Errorf("level 3 = %q", l3.Name)
	}
	if len(l3.Nested) != 0 {
		t.Errorf("level 3 should be leaf, got %d nested", len(l3.Nested))
	}
}

func TestLoadSingleName(t *testing.T) {
	load := Load("Posts", Limit(5))
	if load.Name != "Posts" {
		t.Errorf("Name = %q", load.Name)
	}
	if len(load.Nested) != 0 {
		t.Errorf("Nested = %d, want 0", len(load.Nested))
	}
	if len(load.Mods) != 1 {
		t.Errorf("Mods = %d, want 1", len(load.Mods))
	}
}

// --- LoadMany query building tests ---

func TestLoadManyQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	parentIDs := []any{1, 2, 3}

	// Build the query that LoadMany would build, without executing.
	q := NewQuery(d, "posts")
	q.whereParts = append([]wherePart{{
		clause:      buildInClause(d, "user_id", len(parentIDs)),
		args:        parentIDs,
		conjunction: "AND",
	}}, q.whereParts...)

	query, args := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" WHERE "user_id" IN ($1, $2, $3)`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
	for i, id := range []any{1, 2, 3} {
		if args[i] != id {
			t.Errorf("args[%d] = %v, want %v", i, args[i], id)
		}
	}
}

func TestLoadManyQueryBuildingWithMods(t *testing.T) {
	d := PostgresDialect{}
	parentIDs := []any{10, 20}

	q := NewQuery(d, "comments", Where(`"approved" = ?`, true), OrderBy(`"created_at" DESC`))
	q.whereParts = append([]wherePart{{
		clause:      buildInClause(d, "post_id", len(parentIDs)),
		args:        parentIDs,
		conjunction: "AND",
	}}, q.whereParts...)

	query, args := q.BuildSelect()

	// IN clause comes first, then the user mod.
	want := `SELECT "comments".* FROM "comments" WHERE "post_id" IN ($1, $2) AND "approved" = $3 ORDER BY "created_at" DESC`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
}

func TestLoadManyEmptyParentIDs(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	rows, err := LoadMany(context.Background(), exec, d, "posts", "user_id", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if rows != nil {
		t.Error("expected nil rows for empty parentIDs")
	}
}

func TestLoadManyEmptySlice(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	rows, err := LoadMany(context.Background(), exec, d, "posts", "user_id", []any{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if rows != nil {
		t.Error("expected nil rows for empty parentIDs slice")
	}
}

// --- LoadCount query building tests ---

func TestLoadCountQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	parentIDs := []any{1, 2, 3}

	// Reproduce the query that LoadCount builds internally.
	q := NewQuery(d, "posts")
	q.selectCols = []string{
		d.QuoteIdent("user_id"),
		"COUNT(*) AS __count",
	}
	q.whereParts = append(q.whereParts, wherePart{
		clause:      buildInClause(d, "user_id", len(parentIDs)),
		args:        parentIDs,
		conjunction: "AND",
	})
	q.groupBy = []string{d.QuoteIdent("user_id")}

	query, args := q.BuildSelect()

	want := `SELECT "user_id", COUNT(*) AS __count FROM "posts" WHERE "user_id" IN ($1, $2, $3) GROUP BY "user_id"`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
}

func TestLoadCountEmptyParentIDs(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	result, err := LoadCount(context.Background(), exec, d, "posts", "user_id", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty parentIDs")
	}
}

// --- LoadManyToMany query building tests ---

func TestLoadManyToManyQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	localIDs := []any{1, 2}

	// Reproduce the query LoadManyToMany builds.
	joinAlias := "__jt"
	joinKeyAlias := "__join_key"

	q := NewQuery(d, "tags")
	q.selectCols = append([]string{
		d.QuoteIdent("tags") + ".*",
		joinAlias + "." + d.QuoteIdent("post_id") + " AS " + joinKeyAlias,
	}, q.selectCols...)
	q.joins = append([]joinPart{{
		joinType: "JOIN",
		table:    d.QuoteIdent("post_tags") + " AS " + joinAlias,
		on:       joinAlias + "." + d.QuoteIdent("tag_id") + " = " + d.QuoteIdent("tags") + "." + d.QuoteIdent("id"),
	}}, q.joins...)
	q.whereParts = append([]wherePart{{
		clause:      buildInClauseWithPrefix(d, joinAlias, "post_id", len(localIDs)),
		args:        localIDs,
		conjunction: "AND",
	}}, q.whereParts...)

	query, args := q.BuildSelect()

	// Verify the key parts of the JOIN query.
	if !strings.Contains(query, `JOIN "post_tags" AS __jt`) {
		t.Errorf("missing JOIN clause in: %q", query)
	}
	if !strings.Contains(query, `__jt."tag_id" = "tags"."id"`) {
		t.Errorf("missing ON clause in: %q", query)
	}
	if !strings.Contains(query, `__jt."post_id" IN ($1, $2)`) {
		t.Errorf("missing WHERE IN clause in: %q", query)
	}
	if !strings.Contains(query, `__jt."post_id" AS __join_key`) {
		t.Errorf("missing join key alias in: %q", query)
	}
	if !strings.Contains(query, `"tags".*`) {
		t.Errorf("missing target select in: %q", query)
	}
	if len(args) != 2 {
		t.Errorf("args = %v, want 2 elements", args)
	}
}

func TestLoadManyToManyEmptyLocalIDs(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	rows, alias, err := LoadManyToMany(context.Background(), exec, d,
		"tags", "post_tags", "post_id", "tag_id", "id", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if rows != nil {
		t.Error("expected nil rows for empty localIDs")
	}
	if alias != "" {
		t.Errorf("alias = %q, want empty", alias)
	}
}

// --- LoadManyToManyCount query building tests ---

func TestLoadManyToManyCountQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	localIDs := []any{5, 6, 7}

	q := NewQuery(d, "post_tags")
	q.selectCols = []string{
		d.QuoteIdent("post_id"),
		"COUNT(*) AS __count",
	}
	q.whereParts = append(q.whereParts, wherePart{
		clause:      buildInClause(d, "post_id", len(localIDs)),
		args:        localIDs,
		conjunction: "AND",
	})
	q.groupBy = []string{d.QuoteIdent("post_id")}

	query, args := q.BuildSelect()

	want := `SELECT "post_id", COUNT(*) AS __count FROM "post_tags" WHERE "post_id" IN ($1, $2, $3) GROUP BY "post_id"`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 3 {
		t.Errorf("args len = %d, want 3", len(args))
	}
}

func TestLoadManyToManyCountEmptyIDs(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	result, err := LoadManyToManyCount(context.Background(), exec, d,
		"post_tags", "post_id", "tag_id", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if result != nil {
		t.Error("expected nil for empty localIDs")
	}
}

// --- LoadOne query building ---

func TestLoadOneQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	// We can't call LoadOne without a real executor that returns *sql.Row,
	// but we can verify the SQL it would build.
	var b strings.Builder
	b.WriteString("SELECT * FROM ")
	b.WriteString(d.QuoteIdent("users"))
	b.WriteString(" WHERE ")
	b.WriteString(d.QuoteIdent("id"))
	b.WriteString(" = ")
	b.WriteString(d.Placeholder(1))
	b.WriteString(" LIMIT 1")

	want := `SELECT * FROM "users" WHERE "id" = $1 LIMIT 1`
	if b.String() != want {
		t.Errorf("query = %q, want %q", b.String(), want)
	}
}

// --- Polymorphic query building ---

func TestLoadPolymorphicManyQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	parentIDs := []any{1, 2}

	q := NewQuery(d, "comments")
	q.whereParts = append([]wherePart{
		{
			clause:      d.QuoteIdent("commentable_type") + " = ?",
			args:        []any{"Post"},
			conjunction: "AND",
		},
		{
			clause:      buildInClause(d, "commentable_id", len(parentIDs)),
			args:        parentIDs,
			conjunction: "AND",
		},
	}, q.whereParts...)

	query, args := q.BuildSelect()

	if !strings.Contains(query, `"commentable_type" = $1`) {
		t.Errorf("missing type filter in: %q", query)
	}
	if !strings.Contains(query, `"commentable_id" IN ($2, $3)`) {
		t.Errorf("missing IN clause in: %q", query)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
	if args[0] != "Post" {
		t.Errorf("args[0] = %v, want 'Post'", args[0])
	}
}

func TestLoadPolymorphicManyEmptyIDs(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	rows, err := LoadPolymorphicMany(context.Background(), exec, d,
		"comments", "commentable_type", "Post", "commentable_id", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if rows != nil {
		t.Error("expected nil rows for empty parentIDs")
	}
}

func TestLoadPolymorphicCountEmptyIDs(t *testing.T) {
	d := PostgresDialect{}
	exec := &testExec{}

	result, err := LoadPolymorphicCount(context.Background(), exec, d,
		"comments", "commentable_type", "Post", "commentable_id", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if result != nil {
		t.Error("expected nil for empty parentIDs")
	}
}

// --- BuildInClause edge cases ---

func TestBuildInClauseSingleElement(t *testing.T) {
	d := PostgresDialect{}
	clause := buildInClause(d, "id", 1)
	if clause != `"id" IN (?)` {
		t.Errorf("clause = %q", clause)
	}
}

func TestBuildInClauseWithPrefixEmpty(t *testing.T) {
	d := PostgresDialect{}
	// Empty prefix should behave like buildInClause.
	clause := buildInClauseWithPrefix(d, "", "id", 2)
	want := `"id" IN (?, ?)`
	if clause != want {
		t.Errorf("clause = %q, want %q", clause, want)
	}
}

// --- LoadMany with executor that captures SQL ---

func TestLoadManyExecutorSQL(t *testing.T) {
	d := PostgresDialect{}
	parentIDs := []any{10, 20, 30}

	var capturedQuery string
	var capturedArgs []any
	exec := &testExec{
		err: errors.New("stop"),
		queryFn: func(query string, args []any) {
			capturedQuery = query
			capturedArgs = args
		},
	}

	_, _ = LoadMany(context.Background(), exec, d, "posts", "author_id", parentIDs,
		Where(`"draft" = ?`, false),
		OrderBy(`"title"`),
	)

	want := `SELECT "posts".* FROM "posts" WHERE "author_id" IN ($1, $2, $3) AND "draft" = $4 ORDER BY "title"`
	if capturedQuery != want {
		t.Errorf("query:\n  got  %q\n  want %q", capturedQuery, want)
	}
	if len(capturedArgs) != 4 {
		t.Fatalf("args len = %d, want 4", len(capturedArgs))
	}
	if capturedArgs[3] != false {
		t.Errorf("args[3] = %v, want false", capturedArgs[3])
	}
}

