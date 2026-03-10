package runtime

import (
	"strings"
	"testing"
)

func TestLoadManyWithMods(t *testing.T) {
	// LoadMany now uses the query builder internally, so we can test SQL generation
	// by checking that mods are applied. We can't run actual queries without a DB,
	// but we can verify the Load helper captures mods correctly.

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
