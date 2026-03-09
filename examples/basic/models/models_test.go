package models

import (
	"context"
	"testing"

	"github.com/davidbyttow/sqlgen/runtime"
)

// TestEnumPostStatus verifies enum type, constants, and validation.
func TestEnumPostStatus(t *testing.T) {
	// Constants exist.
	if PostStatusDraft != "draft" {
		t.Errorf("PostStatusDraft = %q", PostStatusDraft)
	}
	if PostStatusPublished != "published" {
		t.Errorf("PostStatusPublished = %q", PostStatusPublished)
	}
	if PostStatusArchived != "archived" {
		t.Errorf("PostStatusArchived = %q", PostStatusArchived)
	}

	// IsValid
	if !PostStatusDraft.IsValid() {
		t.Error("draft should be valid")
	}
	if PostStatus("bogus").IsValid() {
		t.Error("bogus should not be valid")
	}

	// AllValues
	vals := AllPostStatusValues()
	if len(vals) != 3 {
		t.Errorf("AllPostStatusValues() = %d values, want 3", len(vals))
	}

	// Scan/Value round-trip
	var e PostStatus
	if err := e.Scan("published"); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if e != PostStatusPublished {
		t.Errorf("after Scan, got %q", e)
	}
	v, err := e.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if v != "published" {
		t.Errorf("Value() = %v", v)
	}

	// Scan bytes
	if err := e.Scan([]byte("draft")); err != nil {
		t.Fatalf("Scan([]byte): %v", err)
	}
	if e != PostStatusDraft {
		t.Errorf("after Scan([]byte), got %q", e)
	}

	// Scan invalid
	if err := e.Scan("nope"); err == nil {
		t.Error("expected error for invalid value")
	}
	if err := e.Scan(nil); err == nil {
		t.Error("expected error for nil")
	}
}

// TestTableConstants verifies table name constants.
func TestTableConstants(t *testing.T) {
	tests := map[string]string{
		"users":     UserTableName,
		"posts":     PostTableName,
		"tags":      TagTableName,
		"post_tags": PostTagTableName,
	}
	for want, got := range tests {
		if got != want {
			t.Errorf("table name = %q, want %q", got, want)
		}
	}
}

// TestColumnConstants verifies column name constants are correct.
func TestColumnConstants(t *testing.T) {
	if UserColumns.ID != "id" {
		t.Errorf("UserColumns.ID = %q", UserColumns.ID)
	}
	if UserColumns.Email != "email" {
		t.Errorf("UserColumns.Email = %q", UserColumns.Email)
	}
	if PostColumns.Title != "title" {
		t.Errorf("PostColumns.Title = %q", PostColumns.Title)
	}
	if PostColumns.Status != "status" {
		t.Errorf("PostColumns.Status = %q", PostColumns.Status)
	}
	if TagColumns.Name != "name" {
		t.Errorf("TagColumns.Name = %q", TagColumns.Name)
	}
}

// TestQueryBuilders verifies query builders produce valid SQL.
func TestQueryBuilders(t *testing.T) {
	tests := []struct {
		name    string
		query   *runtime.Query
		wantSQL bool
	}{
		{"Users()", Users(), true},
		{"Posts()", Posts(), true},
		{"Tags()", Tags(), true},
		{"PostTags()", PostTags(), true},
		{
			"Users with where",
			Users(UserWhere.Email.EQ("test@example.com")),
			true,
		},
		{
			"Posts with enum filter",
			Posts(PostWhere.Status.EQ(PostStatusPublished)),
			true,
		},
		{
			"Posts with IN",
			Posts(PostWhere.Status.IN(PostStatusDraft, PostStatusArchived)),
			true,
		},
		{
			"Users with limit/offset",
			Users(runtime.Limit(10), runtime.Offset(20)),
			true,
		},
		{
			"Posts with order",
			Posts(runtime.OrderBy("created_at DESC")),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _ := tt.query.BuildSelect()
			if tt.wantSQL && sql == "" {
				t.Error("expected non-empty SQL")
			}
		})
	}
}

// TestWhereClauseBuilders exercises the type-safe where clause API.
func TestWhereClauseBuilders(t *testing.T) {
	// EQ
	q := Users(UserWhere.Email.EQ("alice@example.com"))
	sql, args := q.BuildSelect()
	if len(args) != 1 {
		t.Errorf("EQ args = %d, want 1", len(args))
	}
	if sql == "" {
		t.Error("EQ produced empty SQL")
	}

	// NEQ
	q = Users(UserWhere.Email.NEQ("bob@example.com"))
	_, args = q.BuildSelect()
	if len(args) != 1 {
		t.Errorf("NEQ args = %d, want 1", len(args))
	}

	// IN with multiple values
	q = Posts(PostWhere.Status.IN(PostStatusDraft, PostStatusPublished, PostStatusArchived))
	_, args = q.BuildSelect()
	if len(args) != 3 {
		t.Errorf("IN args = %d, want 3", len(args))
	}

	// IsNull / IsNotNull
	q = Users(UserWhere.Bio.IsNull())
	sql, args = q.BuildSelect()
	if sql == "" {
		t.Error("IsNull produced empty SQL")
	}
	if len(args) != 0 {
		t.Errorf("IsNull args = %d, want 0", len(args))
	}

	q = Users(UserWhere.Bio.IsNotNull())
	sql, _ = q.BuildSelect()
	if sql == "" {
		t.Error("IsNotNull produced empty SQL")
	}

	// LT/GT
	q = Users(UserWhere.Email.LT("z"))
	sql, _ = q.BuildSelect()
	if sql == "" {
		t.Error("LT produced empty SQL")
	}
	q = Users(UserWhere.Email.GT("a"))
	sql, _ = q.BuildSelect()
	if sql == "" {
		t.Error("GT produced empty SQL")
	}
}

// TestNullType verifies the generic Null[T] works correctly.
func TestNullType(t *testing.T) {
	u := &User{
		Bio: runtime.NewNull("hello"),
	}
	if !u.Bio.Valid {
		t.Error("Bio should be valid")
	}
	if u.Bio.Val != "hello" {
		t.Errorf("Bio.Val = %q", u.Bio.Val)
	}

	u.Bio = runtime.Null[string]{}
	if u.Bio.Valid {
		t.Error("Bio should not be valid after zero value")
	}
}

// TestModelStructs verifies struct fields exist and have correct types.
func TestModelStructs(t *testing.T) {
	// User
	u := &User{
		ID:    "uuid-123",
		Email: "test@example.com",
		Name:  "Test",
	}
	if u.ID != "uuid-123" {
		t.Errorf("User.ID = %q", u.ID)
	}
	if u.R != nil {
		t.Error("R should be nil by default")
	}

	// Post with enum field
	p := &Post{
		Title:  "Hello",
		Status: PostStatusDraft,
	}
	if p.Status != PostStatusDraft {
		t.Errorf("Post.Status = %q", p.Status)
	}
}

// TestSliceTypes verifies slice types work.
func TestSliceTypes(t *testing.T) {
	users := UserSlice{
		{ID: "1", Email: "a@b.com"},
		{ID: "2", Email: "c@d.com"},
	}
	if len(users) != 2 {
		t.Errorf("len = %d", len(users))
	}

	posts := PostSlice{
		{ID: "1", Title: "First"},
	}
	if len(posts) != 1 {
		t.Errorf("len = %d", len(posts))
	}
}

// TestOrAndExpr verifies Or and Expr query mods.
func TestOrAndExpr(t *testing.T) {
	// Or
	q := Posts(
		PostWhere.Status.EQ(PostStatusDraft),
		runtime.Or("title = ?", "Hello"),
	)
	sql, args := q.BuildSelect()
	if sql == "" {
		t.Error("Or query is empty")
	}
	if len(args) != 2 {
		t.Errorf("Or args = %d, want 2", len(args))
	}

	// Expr (grouped conditions)
	q = Posts(
		runtime.Expr(
			PostWhere.Status.EQ(PostStatusDraft),
			runtime.Or("title = ?", "Hello"),
		),
	)
	sql, args = q.BuildSelect()
	if sql == "" {
		t.Error("Expr query is empty")
	}
	if len(args) != 2 {
		t.Errorf("Expr args = %d, want 2", len(args))
	}
}

// TestUpdateAllDeleteAll verifies bulk operation SQL building.
func TestUpdateAllDeleteAll(t *testing.T) {
	// UpdateAll
	q := Posts(PostWhere.Status.EQ(PostStatusDraft))
	sql, args := q.BuildUpdateAll(map[string]any{"status": PostStatusArchived})
	if sql == "" {
		t.Error("UpdateAll SQL is empty")
	}
	if len(args) != 2 {
		t.Errorf("UpdateAll args = %d, want 2 (set + where)", len(args))
	}

	// DeleteAll
	q = Posts(PostWhere.Status.EQ(PostStatusArchived))
	sql, args = q.BuildDeleteAll()
	if sql == "" {
		t.Error("DeleteAll SQL is empty")
	}
	if len(args) != 1 {
		t.Errorf("DeleteAll args = %d, want 1", len(args))
	}
}

// TestEagerLoadRequest verifies Load() helper.
func TestEagerLoadRequest(t *testing.T) {
	load := runtime.Load("Posts")
	if load.Name != "Posts" {
		t.Errorf("Name = %q", load.Name)
	}
	if len(load.Nested) != 0 {
		t.Error("no nesting expected")
	}

	// Dot notation
	load = runtime.Load("Posts.Tags")
	if load.Name != "Posts" {
		t.Errorf("Name = %q", load.Name)
	}
	if len(load.Nested) != 1 {
		t.Fatal("expected 1 nested")
	}
	if load.Nested[0].Name != "Tags" {
		t.Errorf("Nested[0].Name = %q", load.Nested[0].Name)
	}
}

// TestDistinctAndJoins verifies distinct and join query mods.
func TestDistinctAndJoins(t *testing.T) {
	q := Posts(runtime.Distinct())
	sql, _ := q.BuildSelect()
	if sql == "" {
		t.Error("Distinct query is empty")
	}

	q = Posts(runtime.LeftJoin("users", "users.id = posts.author_id"))
	sql, _ = q.BuildSelect()
	if sql == "" {
		t.Error("LeftJoin query is empty")
	}
}

// TestHookRegistration verifies hooks can be added.
func TestHookRegistration(t *testing.T) {
	// Just verify it doesn't panic. Hooks now receive typed model pointers.
	AddUserHook(runtime.BeforeInsert, func(ctx context.Context, exec runtime.Executor, model *User) (context.Context, error) {
		return ctx, nil
	})
	AddPostHook(runtime.AfterInsert, func(ctx context.Context, exec runtime.Executor, model *Post) (context.Context, error) {
		return ctx, nil
	})
}
