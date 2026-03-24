package schema

import "testing"

func findRel(rels []*Relationship, relType RelationType, foreignTable string) *Relationship {
	for _, r := range rels {
		if r.Type == relType && r.ForeignTable == foreignTable {
			return r
		}
	}
	return nil
}

func TestResolveRelationships_BelongsToAndHasMany(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:    "organizations",
				Columns: []*Column{{Name: "id", DBType: "uuid"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:    "users",
				Columns: []*Column{{Name: "id", DBType: "uuid"}, {Name: "org_id", DBType: "uuid"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []*ForeignKey{
					{Name: "fk_org", Columns: []string{"org_id"}, RefTable: "organizations", RefColumns: []string{"id"}},
				},
			},
		},
	}

	ResolveRelationships(s)

	users := s.Tables[1]
	orgs := s.Tables[0]

	// Users belongs to organizations.
	bt := findRel(users.Relationships, RelBelongsTo, "organizations")
	if bt == nil {
		t.Fatal("users should have BelongsTo organizations")
	}
	if bt.Columns[0] != "org_id" {
		t.Errorf("BelongsTo columns = %v, want [org_id]", bt.Columns)
	}

	// Organizations has many users.
	hm := findRel(orgs.Relationships, RelHasMany, "users")
	if hm == nil {
		t.Fatal("organizations should have HasMany users")
	}
}

func TestResolveRelationships_HasOne(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:       "users",
				Columns:    []*Column{{Name: "id", DBType: "uuid"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:       "profiles",
				Columns:    []*Column{{Name: "id", DBType: "uuid"}, {Name: "user_id", DBType: "uuid"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
				Unique: []*UniqueConstraint{
					{Columns: []string{"user_id"}},
				},
			},
		},
	}

	ResolveRelationships(s)

	users := s.Tables[0]
	ho := findRel(users.Relationships, RelHasOne, "profiles")
	if ho == nil {
		t.Fatal("users should have HasOne profiles (FK column is unique)")
	}
}

func TestResolveRelationships_ManyToMany(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:       "posts",
				Columns:    []*Column{{Name: "id", DBType: "uuid"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:       "tags",
				Columns:    []*Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:    "post_tags",
				Columns: []*Column{{Name: "post_id", DBType: "uuid"}, {Name: "tag_id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"post_id", "tag_id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"post_id"}, RefTable: "posts", RefColumns: []string{"id"}},
					{Columns: []string{"tag_id"}, RefTable: "tags", RefColumns: []string{"id"}},
				},
			},
		},
	}

	ResolveRelationships(s)

	posts := s.Tables[0]
	tags := s.Tables[1]

	// Posts has many tags through post_tags.
	pm := findRel(posts.Relationships, RelManyToMany, "tags")
	if pm == nil {
		t.Fatal("posts should have ManyToMany tags")
	}
	if pm.JoinTable != "post_tags" {
		t.Errorf("join table = %q, want post_tags", pm.JoinTable)
	}

	// Tags has many posts through post_tags.
	tm := findRel(tags.Relationships, RelManyToMany, "posts")
	if tm == nil {
		t.Fatal("tags should have ManyToMany posts")
	}
	if tm.JoinTable != "post_tags" {
		t.Errorf("join table = %q, want post_tags", tm.JoinTable)
	}
}

func TestResolveRelationships_SelfReferencing(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:       "categories",
				Columns:    []*Column{{Name: "id", DBType: "integer"}, {Name: "parent_id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"parent_id"}, RefTable: "categories", RefColumns: []string{"id"}},
				},
			},
		},
	}

	ResolveRelationships(s)

	cats := s.Tables[0]
	bt := findRel(cats.Relationships, RelBelongsTo, "categories")
	if bt == nil {
		t.Fatal("categories should have BelongsTo self")
	}

	hm := findRel(cats.Relationships, RelHasMany, "categories")
	if hm == nil {
		t.Fatal("categories should have HasMany self (children)")
	}
}

func TestIsJoinTable(t *testing.T) {
	tests := []struct {
		name string
		table *Table
		want  bool
	}{
		{
			"valid join table",
			&Table{
				PrimaryKey: &PrimaryKey{Columns: []string{"a_id", "b_id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"a_id"}},
					{Columns: []string{"b_id"}},
				},
			},
			true,
		},
		{
			"not enough FKs",
			&Table{
				PrimaryKey: &PrimaryKey{Columns: []string{"a_id", "b_id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"a_id"}},
				},
			},
			false,
		},
		{
			"PK has extra columns",
			&Table{
				PrimaryKey: &PrimaryKey{Columns: []string{"a_id", "b_id", "extra"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"a_id"}},
					{Columns: []string{"b_id"}},
				},
			},
			false,
		},
		{
			"no PK",
			&Table{
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"a_id"}},
					{Columns: []string{"b_id"}},
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isJoinTable(tt.table)
			if got != tt.want {
				t.Errorf("isJoinTable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinTable3FKNotDetected(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:       "a",
				Columns:    []*Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:       "b",
				Columns:    []*Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:       "c",
				Columns:    []*Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:    "a_b_c",
				Columns: []*Column{{Name: "a_id"}, {Name: "b_id"}, {Name: "c_id"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"a_id", "b_id", "c_id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}},
					{Columns: []string{"b_id"}, RefTable: "b", RefColumns: []string{"id"}},
					{Columns: []string{"c_id"}, RefTable: "c", RefColumns: []string{"id"}},
				},
			},
		},
	}

	if isJoinTable(s.Tables[3]) {
		t.Error("table with 3 FKs should NOT be detected as a join table")
	}

	ResolveRelationships(s)

	abc := s.Tables[3]
	btCount := 0
	for _, r := range abc.Relationships {
		if r.Type == RelBelongsTo {
			btCount++
		}
	}
	if btCount != 3 {
		t.Errorf("expected 3 BelongsTo relationships, got %d", btCount)
	}

	for _, tbl := range s.Tables[:3] {
		for _, r := range tbl.Relationships {
			if r.Type == RelManyToMany {
				t.Errorf("table %q should not have ManyToMany relationship", tbl.Name)
			}
		}
	}
}

func TestSelfRefJoinTable(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:       "users",
				Columns:    []*Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:    "friendships",
				Columns: []*Column{{Name: "user_id", DBType: "integer"}, {Name: "friend_id", DBType: "integer"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"user_id", "friend_id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
					{Columns: []string{"friend_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
	}

	if !isJoinTable(s.Tables[1]) {
		t.Fatal("friendships should be detected as a join table")
	}

	ResolveRelationships(s)

	users := s.Tables[0]
	m2mCount := 0
	for _, r := range users.Relationships {
		if r.Type == RelManyToMany {
			m2mCount++
			if r.ForeignTable != "users" {
				t.Errorf("ManyToMany foreign table = %q, want users", r.ForeignTable)
			}
			if r.JoinTable != "friendships" {
				t.Errorf("ManyToMany join table = %q, want friendships", r.JoinTable)
			}
		}
	}
	if m2mCount != 2 {
		t.Errorf("expected 2 ManyToMany rels on users (self-referencing), got %d", m2mCount)
	}
}

func TestMissingTableFK(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:       "orders",
				Columns:    []*Column{{Name: "id"}, {Name: "customer_id"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []*ForeignKey{
					{Columns: []string{"customer_id"}, RefTable: "customers", RefColumns: []string{"id"}},
				},
			},
		},
	}

	ResolveRelationships(s)

	orders := s.Tables[0]
	if len(orders.Relationships) != 0 {
		t.Errorf("expected 0 relationships (FK target missing), got %d", len(orders.Relationships))
	}
}

func TestCompositeFKColumns(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name:    "tenant_resources",
				Columns: []*Column{{Name: "tenant_id"}, {Name: "resource_id"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"tenant_id", "resource_id"}},
			},
			{
				Name:    "allocations",
				Columns: []*Column{{Name: "id"}, {Name: "tenant_id"}, {Name: "resource_id"}},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []*ForeignKey{
					{
						Columns:    []string{"tenant_id", "resource_id"},
						RefTable:   "tenant_resources",
						RefColumns: []string{"tenant_id", "resource_id"},
					},
				},
			},
		},
	}

	ResolveRelationships(s)

	allocations := s.Tables[1]
	bt := findRel(allocations.Relationships, RelBelongsTo, "tenant_resources")
	if bt == nil {
		t.Fatal("allocations should have BelongsTo tenant_resources")
	}
	if len(bt.Columns) != 2 {
		t.Fatalf("expected 2 FK columns, got %d", len(bt.Columns))
	}
	if bt.Columns[0] != "tenant_id" || bt.Columns[1] != "resource_id" {
		t.Errorf("FK columns = %v, want [tenant_id resource_id]", bt.Columns)
	}

	tr := s.Tables[0]
	hm := findRel(tr.Relationships, RelHasMany, "allocations")
	if hm == nil {
		t.Fatal("tenant_resources should have HasMany allocations")
	}
	if len(hm.ForeignColumns) != 2 {
		t.Errorf("HasMany foreign columns = %v, want 2 columns", hm.ForeignColumns)
	}
}

func TestResolvePolymorphic(t *testing.T) {
	s := &Schema{
		Tables: []*Table{
			{
				Name: "users",
				Columns: []*Column{
					{Name: "id", DBType: "integer"},
					{Name: "name", DBType: "text"},
				},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name: "posts",
				Columns: []*Column{
					{Name: "id", DBType: "integer"},
					{Name: "title", DBType: "text"},
				},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name: "comments",
				Columns: []*Column{
					{Name: "id", DBType: "integer"},
					{Name: "body", DBType: "text"},
					{Name: "commentable_type", DBType: "text"},
					{Name: "commentable_id", DBType: "integer"},
				},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	ResolvePolymorphic(s, []PolymorphicDef{
		{
			Table:      "comments",
			TypeColumn: "commentable_type",
			IDColumn:   "commentable_id",
			Targets:    map[string]string{"User": "users", "Post": "posts"},
		},
	})

	comments := s.Tables[2]
	users := s.Tables[0]
	posts := s.Tables[1]

	// Comments should have two PolymorphicOne relationships
	polyOnes := 0
	for _, r := range comments.Relationships {
		if r.Type == RelPolymorphicOne {
			polyOnes++
			if r.TypeColumn != "commentable_type" {
				t.Errorf("expected TypeColumn 'commentable_type', got %q", r.TypeColumn)
			}
			if r.IDColumn != "commentable_id" {
				t.Errorf("expected IDColumn 'commentable_id', got %q", r.IDColumn)
			}
		}
	}
	if polyOnes != 2 {
		t.Errorf("expected 2 PolymorphicOne rels on comments, got %d", polyOnes)
	}

	// Users should have a PolymorphicMany for comments
	userPolyMany := findRel(users.Relationships, RelPolymorphicMany, "comments")
	if userPolyMany == nil {
		t.Fatal("users should have PolymorphicMany to comments")
	}
	if userPolyMany.TypeValue != "User" {
		t.Errorf("expected TypeValue 'User', got %q", userPolyMany.TypeValue)
	}
	if userPolyMany.TypeColumn != "commentable_type" {
		t.Errorf("expected TypeColumn 'commentable_type', got %q", userPolyMany.TypeColumn)
	}

	// Posts should have a PolymorphicMany for comments
	postPolyMany := findRel(posts.Relationships, RelPolymorphicMany, "comments")
	if postPolyMany == nil {
		t.Fatal("posts should have PolymorphicMany to comments")
	}
	if postPolyMany.TypeValue != "Post" {
		t.Errorf("expected TypeValue 'Post', got %q", postPolyMany.TypeValue)
	}
}
