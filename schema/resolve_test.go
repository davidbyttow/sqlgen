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
