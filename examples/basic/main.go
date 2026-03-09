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
	"github.com/davidbyttow/sqlgen/runtime"
)

func main() {
	// --- Enum usage ---
	// The post_status enum becomes a type-safe Go string type with constants.
	status := models.PostStatusDraft
	fmt.Println("Status:", status)
	fmt.Println("Valid?", status.IsValid())
	fmt.Println("All values:", models.AllPostStatusValues())

	// --- Model structs ---
	// Each table gets a struct with typed fields. Nullable columns use runtime.Null[T].
	user := models.User{
		Email: "alice@example.com",
		Name:  "Alice",
		Bio:   runtime.NewNull("Writes Go for fun."),
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
	// These return runtime.QueryMod values you can compose freely.
	q := models.Users(
		models.UserWhere.Email.EQ("alice@example.com"),
		runtime.Limit(1),
	)
	sql, args := q.BuildSelect()
	fmt.Printf("\nFind user by email:\n  SQL:  %s\n  Args: %v\n", sql, args)

	// Nullable columns get IsNull/IsNotNull helpers.
	q = models.Users(
		models.UserWhere.Bio.IsNotNull(),
		runtime.OrderBy("created_at DESC"),
	)
	sql, args = q.BuildSelect()
	fmt.Printf("\nUsers with bios:\n  SQL:  %s\n  Args: %v\n", sql, args)

	// --- Composing query mods ---
	// Stack multiple where clauses (ANDed), ordering, limit, offset.
	q = models.Posts(
		models.PostWhere.Status.EQ(models.PostStatusPublished),
		models.PostWhere.AuthorID.EQ("some-uuid"),
		runtime.OrderBy("published_at DESC"),
		runtime.Limit(10),
		runtime.Offset(20),
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
	insertSQL, insertArgs := runtime.BuildInsert(
		runtime.PostgresDialect{},
		models.PostTableName,
		[]string{"author_id", "title", "body", "status"},
		[]any{post.AuthorID, post.Title, post.Body, post.Status},
		[]string{"id", "author_id", "title", "body", "status", "created_at", "published_at"},
	)
	fmt.Printf("\nInsert post:\n  SQL:  %s\n  Args: %v\n", insertSQL, insertArgs)

	// --- Hook registration ---
	// Each model has an AddXxxHook function for lifecycle events.
	models.AddUserHook(runtime.BeforeInsert, func(ctx context.Context) (context.Context, error) {
		fmt.Println("\n[hook] BeforeInsert fired for User")
		return ctx, nil
	})

	models.AddPostHook(runtime.AfterInsert, func(ctx context.Context) (context.Context, error) {
		fmt.Println("[hook] AfterInsert fired for Post")
		return ctx, nil
	})

	// Hooks can be skipped per-call via context.
	ctx := runtime.SkipHooks(context.Background())
	fmt.Println("Hooks skipped?", runtime.ShouldSkipHooks(ctx))

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
	//   users.LoadRelations(ctx, db, runtime.Load("Posts"))
	//
	// Nested loading with dot notation:
	//   users.LoadRelations(ctx, db, runtime.Load("Posts.Tags"))
	//
	// Multiple relationships at once:
	//   posts, _ := models.AllPosts(ctx, db)
	//   posts.LoadRelations(ctx, db, runtime.Load("User"), runtime.Load("Tags"))

	// Demonstrate the Load helper constructs the right request.
	load := runtime.Load("Posts.Tags")
	fmt.Printf("\nEager load request: Name=%q, Nested=%v\n", load.Name, load.Nested != nil)
	if len(load.Nested) > 0 {
		fmt.Printf("  Nested: Name=%q\n", load.Nested[0].Name)
	}
}
