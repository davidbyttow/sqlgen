package sqlgen

import (
	"strings"
	"testing"
)

func TestPreloadQueryBuild(t *testing.T) {
	def := PreloadDef{
		Name:     "Organization",
		Table:    "organizations",
		JoinCond: `"organizations"."id" = "users"."org_id"`,
		Columns: []string{
			`"organizations"."id"`,
			`"organizations"."name"`,
		},
	}

	q := NewQuery(PostgresDialect{}, "users", Preload(def))
	sql, _ := q.BuildSelect()

	// Should include preload columns.
	if !strings.Contains(sql, `"organizations"."id"`) {
		t.Errorf("missing preload column in SELECT: %s", sql)
	}
	if !strings.Contains(sql, `"organizations"."name"`) {
		t.Errorf("missing preload column in SELECT: %s", sql)
	}

	// Should have LEFT JOIN.
	if !strings.Contains(sql, `LEFT JOIN "organizations"`) {
		t.Errorf("missing LEFT JOIN: %s", sql)
	}

	// Should have ON clause.
	if !strings.Contains(sql, `"organizations"."id" = "users"."org_id"`) {
		t.Errorf("missing ON clause: %s", sql)
	}
}

func TestPreloadWithWhere(t *testing.T) {
	def := PreloadDef{
		Name:     "Organization",
		Table:    "organizations",
		JoinCond: `"organizations"."id" = "users"."org_id"`,
		Columns:  []string{`"organizations"."id"`},
	}

	q := NewQuery(PostgresDialect{}, "users",
		Where(`"email" = ?`, "test@example.com"),
		Preload(def),
		Limit(10),
	)
	sql, args := q.BuildSelect()

	if !strings.Contains(sql, `LEFT JOIN "organizations"`) {
		t.Errorf("missing LEFT JOIN: %s", sql)
	}
	if !strings.Contains(sql, `"email" = $1`) {
		t.Errorf("missing WHERE clause: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT $2") {
		t.Errorf("missing LIMIT: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("args = %d, want 2", len(args))
	}
}

func TestPreloads(t *testing.T) {
	def := PreloadDef{Name: "Org", Table: "orgs"}
	q := NewQuery(PostgresDialect{}, "users", Preload(def))

	preloads := q.Preloads()
	if len(preloads) != 1 {
		t.Fatalf("preloads = %d, want 1", len(preloads))
	}
	if preloads[0].Name != "Org" {
		t.Errorf("name = %q, want Org", preloads[0].Name)
	}
}

func TestNoPreloads(t *testing.T) {
	q := NewQuery(PostgresDialect{}, "users")
	if len(q.Preloads()) != 0 {
		t.Error("expected no preloads")
	}
}
