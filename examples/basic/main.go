// This example demonstrates the sqlgen generated API.
// It builds queries and prints the resulting SQL without connecting to a database.
//
// To generate the models package, run from this directory:
//
//	go run ../../cmd/sqlgen generate
package main

import (
	"context"
	"fmt"

	"github.com/davidbyttow/sqlgen/examples/basic/models"
	"github.com/davidbyttow/sqlgen"
)

func main() {
	// --- Enum usage ---
	// The post_status enum becomes a type-safe Go string type with constants.
	status := models.PostStatusDraft
	fmt.Println("Status:", status)
	fmt.Println("Valid?", status.IsValid())
	fmt.Println("All values:", models.AllPostStatusValues())

	// --- Model structs ---
	// Each table gets a struct with typed fields. Nullable columns use sqlgen.Null[T].
	user := models.User{
		Email: "alice@example.com",
		Name:  "Alice",
		Bio:   sqlgen.NewNull("Writes Go for fun."),
	}
	fmt.Printf("\nUser: %s <%s>\n", user.Name, user.Email)
	fmt.Printf("Bio: %s\n", user.Bio)

	post := models.Post{
		AuthorID: user.ID,
		Title:    "Getting Started with sqlgen",
		Body:     "This is the body of the post.",
		Status:   models.PostStatusPublished,
	}
	fmt.Printf("Post: %q [%s]\n", post.Title, post.Status)

	// --- Type-safe where clauses ---
	// Each table gets a <Model>Where var with per-column filter methods.
	// These return sqlgen.QueryMod values you can compose freely.
	q := models.Users(
		models.UserWhere.Email.EQ("alice@example.com"),
		sqlgen.Limit(1),
	)
	sql, args := q.BuildSelect()
	fmt.Printf("\nFind user by email:\n  SQL:  %s\n  Args: %v\n", sql, args)

	// Nullable columns get IsNull/IsNotNull helpers.
	q = models.Users(
		models.UserWhere.Bio.IsNotNull(),
		sqlgen.OrderBy("created_at DESC"),
	)
	sql, args = q.BuildSelect()
	fmt.Printf("\nUsers with bios:\n  SQL:  %s\n  Args: %v\n", sql, args)

	// --- Composing query mods ---
	// Stack multiple where clauses (ANDed), ordering, limit, offset.
	q = models.Posts(
		models.PostWhere.Status.EQ(models.PostStatusPublished),
		models.PostWhere.AuthorID.EQ("some-uuid"),
		sqlgen.OrderBy("published_at DESC"),
		sqlgen.Limit(10),
		sqlgen.Offset(20),
	)
	sql, args = q.BuildSelect()
	fmt.Printf("\nPublished posts by author (paginated):\n  SQL:  %s\n  Args: %v\n", sql, args)

	// IN clause with multiple values.
	q = models.Posts(
		models.PostWhere.Status.IN(models.PostStatusDraft, models.PostStatusArchived),
	)
	sql, args = q.BuildSelect()
	fmt.Printf("\nDraft or archived posts:\n  SQL:  %s\n  Args: %v\n", sql, args)

	// --- Insert SQL ---
	// Model.Insert() runs against an Executor (db or tx). Here we just show the shape.
	insertSQL, insertArgs := sqlgen.BuildInsert(
		sqlgen.PostgresDialect{},
		models.PostTableName,
		[]string{"author_id", "title", "body", "status"},
		[]any{post.AuthorID, post.Title, post.Body, post.Status},
		[]string{"id", "author_id", "title", "body", "status", "created_at", "published_at"},
	)
	fmt.Printf("\nInsert post:\n  SQL:  %s\n  Args: %v\n", insertSQL, insertArgs)

	// --- Hook registration ---
	// Each model has an AddXxxHook function for lifecycle events.
	// Hooks receive a typed model pointer so you can inspect or modify it.
	models.AddUserHook(sqlgen.BeforeInsert, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		fmt.Printf("\n[hook] BeforeInsert fired for User: %s\n", model.Email)
		return ctx, nil
	})

	models.AddPostHook(sqlgen.AfterInsert, func(ctx context.Context, exec sqlgen.Executor, model *models.Post) (context.Context, error) {
		fmt.Printf("[hook] AfterInsert fired for Post: %s\n", model.Title)
		return ctx, nil
	})

	// Hooks can be skipped per-call via context.
	ctx := sqlgen.SkipHooks(context.Background())
	fmt.Println("Hooks skipped?", sqlgen.ShouldSkipHooks(ctx))

	// --- Column name constants ---
	// Each model has a <Model>Columns var for safe column references.
	fmt.Printf("\nUser columns: ID=%q, Email=%q, CreatedAt=%q\n",
		models.UserColumns.ID, models.UserColumns.Email, models.UserColumns.CreatedAt)

	// --- Eager Loading API ---
	// After fetching models, use .LoadRelations() to batch-load relationships.
	// This avoids N+1 queries by using WHERE IN (...) for the entire slice.
	//
	// Example (requires a real DB connection):
	//   users, _ := models.AllUsers(ctx, db)
	//   users.LoadRelations(ctx, db, sqlgen.Load("Posts"))
	//
	// Nested loading with dot notation:
	//   users.LoadRelations(ctx, db, sqlgen.Load("Posts.Tags"))
	//
	// Multiple relationships at once:
	//   posts, _ := models.AllPosts(ctx, db)
	//   posts.LoadRelations(ctx, db, sqlgen.Load("User"), sqlgen.Load("Tags"))

	// Demonstrate the Load helper constructs the right request.
	load := sqlgen.Load("Posts.Tags")
	fmt.Printf("\nEager load request: Name=%q, Nested=%v\n", load.Name, load.Nested != nil)
	if len(load.Nested) > 0 {
		fmt.Printf("  Nested: Name=%q\n", load.Nested[0].Name)
	}
}
