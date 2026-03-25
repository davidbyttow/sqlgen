package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/davidbyttow/sqlgen"
	"github.com/davidbyttow/sqlgen/examples/acceptance/models"
	"github.com/davidbyttow/sqlgen/fake"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const dsn = "postgres://sqlgen:sqlgen@localhost:5433/sqlgen_acceptance?sslmode=disable"

var (
	passed      int
	failed      int
	sectionName string
)

func section(name string) {
	sectionName = name
	fmt.Printf("\n\033[1;34m=== %s ===\033[0m\n", name)
}

func check(name string, err error) {
	if err != nil {
		fmt.Printf("  \033[31m✗ %s: %v\033[0m\n", name, err)
		failed++
	} else {
		fmt.Printf("  \033[32m✓ %s\033[0m\n", name)
		passed++
	}
}

func assert(name string, ok bool, msgAndArgs ...any) {
	if !ok {
		msg := ""
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(": "+msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		fmt.Printf("  \033[31m✗ %s%s\033[0m\n", name, msg)
		failed++
	} else {
		fmt.Printf("  \033[32m✓ %s\033[0m\n", name)
		passed++
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func acceptanceDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func dockerCompose(args ...string) error {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = acceptanceDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForDB(maxRetries int) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		err = db.Ping()
		if err == nil {
			return db, nil
		}
		db.Close()
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("could not connect to database after %d retries: %w", maxRetries, err)
}

func resetSchema(db *sql.DB) error {
	schemaPath := filepath.Join(acceptanceDir(), "schema.sql")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("reading schema.sql: %w", err)
	}

	// Drop all tables first (reverse dependency order).
	drops := []string{
		"DROP VIEW IF EXISTS published_posts CASCADE",
		"DROP TABLE IF EXISTS post_tags CASCADE",
		"DROP TABLE IF EXISTS comments CASCADE",
		"DROP TABLE IF EXISTS posts CASCADE",
		"DROP TABLE IF EXISTS profiles CASCADE",
		"DROP TABLE IF EXISTS tags CASCADE",
		"DROP TABLE IF EXISTS categories CASCADE",
		"DROP TABLE IF EXISTS users CASCADE",
		"DROP TYPE IF EXISTS post_status CASCADE",
		"DROP TYPE IF EXISTS comment_kind CASCADE",
	}
	for _, d := range drops {
		if _, err := db.Exec(d); err != nil {
			return fmt.Errorf("drop: %w", err)
		}
	}

	_, err = db.Exec(string(data))
	if err != nil {
		return fmt.Errorf("executing schema.sql: %w", err)
	}
	return nil
}

// truncateAll removes all data but keeps schema.
func truncateAll(db *sql.DB) {
	tables := []string{"comments", "post_tags", "posts", "profiles", "tags", "categories", "users"}
	for _, t := range tables {
		db.Exec("DELETE FROM " + t)
	}
}

// insertTestUser inserts a user with known fields and returns it.
func insertTestUser(ctx context.Context, db *sql.DB, email, name string) *models.User {
	u := &models.User{
		Email:   email,
		Name:    name,
		IsAdmin: false,
	}
	must(u.Insert(ctx, db))
	return u
}

func main() {
	fmt.Println("\033[1;36m--- sqlgen acceptance test ---\033[0m")

	// Start Docker.
	fmt.Println("Starting Docker containers...")
	if err := dockerCompose("up", "-d", "--wait"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start Docker: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure Docker is running.\n")
		os.Exit(1)
	}
	defer func() {
		fmt.Println("\nStopping Docker containers...")
		dockerCompose("down", "-v")
	}()

	// Wait for Postgres.
	fmt.Println("Waiting for Postgres...")
	db, err := waitForDB(30)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Load schema.
	fmt.Println("Loading schema...")
	if err := resetSchema(db); err != nil {
		fmt.Fprintf(os.Stderr, "Schema load failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	testEnums()
	testCRUDInsert(ctx, db)
	testCRUDRead(ctx, db)
	testCRUDUpdate(ctx, db)
	testCRUDUpsert(ctx, db)
	testCRUDDelete(ctx, db)
	testQueryBuilder(ctx, db)
	testTypeSafeWhere(ctx, db)
	testRelationships(ctx, db)
	testEagerLoading(ctx, db)
	testPreloading(ctx, db)
	testHooks(ctx, db)
	testEachCursor(ctx, db)
	testNullType(ctx, db)
	testCachedExecutor(ctx, db)
	testErrorHandling(ctx, db)
	testColumnFiltering(ctx, db)
	testFactories(ctx, db)
	testDebugExecutor(ctx, db)
	testBindScan(ctx, db)

	// Summary.
	fmt.Printf("\n\033[1;36m--- Results: %d passed, %d failed ---\033[0m\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// 1. Enums
// ---------------------------------------------------------------------------

func testEnums() {
	section("Enums")

	// PostStatus
	assert("PostStatusDraft value", string(models.PostStatusDraft) == "draft")
	assert("PostStatusPublished value", string(models.PostStatusPublished) == "published")
	assert("PostStatusArchived value", string(models.PostStatusArchived) == "archived")
	assert("PostStatusDraft.IsValid", models.PostStatusDraft.IsValid())
	assert("PostStatusDraft.String", models.PostStatusDraft.String() == "draft")
	assert("Invalid PostStatus", !models.PostStatus("invalid").IsValid())
	allPS := models.AllPostStatusValues()
	assert("AllPostStatusValues length", len(allPS) == 3)

	// CommentKind
	assert("CommentKindReview value", string(models.CommentKindReview) == "review")
	assert("CommentKindReply value", string(models.CommentKindReply) == "reply")
	assert("CommentKindNote value", string(models.CommentKindNote) == "note")
	assert("CommentKindNote.IsValid", models.CommentKindNote.IsValid())
	allCK := models.AllCommentKindValues()
	assert("AllCommentKindValues length", len(allCK) == 3)
}

// ---------------------------------------------------------------------------
// 2. CRUD Insert
// ---------------------------------------------------------------------------

func testCRUDInsert(ctx context.Context, db *sql.DB) {
	section("CRUD Insert")
	truncateAll(db)

	// Insert user.
	u := &models.User{
		Email:   "alice@example.com",
		Name:    "Alice",
		Bio:     sqlgen.NewNull("A bio"),
		Age:     sqlgen.NewNull(int32(30)),
		IsAdmin: true,
	}
	check("Insert user", u.Insert(ctx, db))
	assert("User ID populated", u.ID != "", "got empty ID")
	// CreatedAt is sent as zero time by generated code (no auto-set).
	// Verify the insert succeeded and ID was populated. CreatedAt will be zero
	// unless we explicitly set it or use the factory (which calls fake.Time()).
	assert("User Email round-tripped", u.Email == "alice@example.com")

	// Insert category (no parent).
	cat := &models.Category{Name: "Tech"}
	check("Insert category", cat.Insert(ctx, db))
	assert("Category ID populated", cat.ID != 0)

	// Insert post.
	p := &models.Post{
		AuthorID:   u.ID,
		CategoryID: sqlgen.NewNull(cat.ID),
		Title:      "First Post",
		Body:       "Hello world",
		Status:     models.PostStatusDraft,
	}
	check("Insert post", p.Insert(ctx, db))
	assert("Post ID populated", p.ID != "")

	// Insert tags.
	tag1 := &models.Tag{Name: "go"}
	tag2 := &models.Tag{Name: "sql"}
	check("Insert tag1", tag1.Insert(ctx, db))
	check("Insert tag2", tag2.Insert(ctx, db))
	assert("Tag1 ID populated", tag1.ID != 0)

	// Insert profile.
	prof := &models.Profile{
		UserID:    u.ID,
		AvatarURL: sqlgen.NewNull("https://example.com/avatar.png"),
		Website:   sqlgen.NewNull("https://alice.dev"),
	}
	check("Insert profile", prof.Insert(ctx, db))
	assert("Profile ID populated", prof.ID != "")

	// Insert comment (polymorphic).
	comment := &models.Comment{
		AuthorID:        u.ID,
		CommentableType: "Post",
		CommentableID:   p.ID,
		Kind:            models.CommentKindReview,
		Body:            "Great post!",
	}
	check("Insert comment", comment.Insert(ctx, db))
	assert("Comment ID populated", comment.ID != "")

	// InsertAll batch.
	users := models.UserSlice{
		{Email: "bob@example.com", Name: "Bob"},
		{Email: "carol@example.com", Name: "Carol"},
	}
	check("InsertAll users", users.InsertAll(ctx, db))
	assert("InsertAll: user[0] ID populated", users[0].ID != "")
	assert("InsertAll: user[1] ID populated", users[1].ID != "")
}

// ---------------------------------------------------------------------------
// 3. CRUD Read
// ---------------------------------------------------------------------------

func testCRUDRead(ctx context.Context, db *sql.DB) {
	section("CRUD Read")
	truncateAll(db)

	u := insertTestUser(ctx, db, "read@example.com", "Reader")

	// FindByPK.
	found, err := models.FindUserByPK(ctx, db, u.ID)
	check("FindUserByPK", err)
	assert("FindUserByPK correct email", found.Email == "read@example.com")

	// AllUsers with mods.
	insertTestUser(ctx, db, "read2@example.com", "Reader2")
	all, err := models.AllUsers(ctx, db, sqlgen.OrderBy("\"email\" ASC"))
	check("AllUsers", err)
	assert("AllUsers count", len(all) == 2, "got %d", len(all))

	// AllUsers with Limit.
	limited, err := models.AllUsers(ctx, db, sqlgen.Limit(1), sqlgen.OrderBy("\"email\" ASC"))
	check("AllUsers with Limit", err)
	assert("AllUsers Limit=1 returns 1", len(limited) == 1)

	// Count.
	count, err := models.CountUsers(ctx, db)
	check("CountUsers", err)
	assert("CountUsers == 2", count == 2, "got %d", count)

	// Exists.
	exists, err := models.UserExists(ctx, db, u.ID)
	check("UserExists", err)
	assert("UserExists true", exists)

	notExists, err := models.UserExists(ctx, db, "00000000-0000-0000-0000-000000000000")
	check("UserExists (missing)", err)
	assert("UserExists false for missing", !notExists)
}

// ---------------------------------------------------------------------------
// 4. CRUD Update
// ---------------------------------------------------------------------------

func testCRUDUpdate(ctx context.Context, db *sql.DB) {
	section("CRUD Update")
	truncateAll(db)

	u := insertTestUser(ctx, db, "update@example.com", "Updater")

	// Update single.
	u.Name = "Updated Name"
	u.Bio.Set("New bio")
	check("Update user", u.Update(ctx, db))

	reloaded, err := models.FindUserByPK(ctx, db, u.ID)
	check("FindUserByPK after update", err)
	assert("Name updated", reloaded.Name == "Updated Name")
	assert("Bio updated", reloaded.Bio.Valid && reloaded.Bio.Val == "New bio")

	// UpdateAll.
	insertTestUser(ctx, db, "update2@example.com", "Updater2")
	n, err := models.UpdateAllUsers(ctx, db, map[string]any{"is_admin": true})
	check("UpdateAllUsers", err)
	assert("UpdateAllUsers affected 2", n == 2, "got %d", n)

	// Slice.UpdateAll.
	all, _ := models.AllUsers(ctx, db)
	n, err = all.UpdateAll(ctx, db, map[string]any{"is_admin": false})
	check("Slice.UpdateAll", err)
	assert("Slice.UpdateAll affected all", n == int64(len(all)))

	// Reload.
	u.Name = "stale"
	check("Reload", u.Reload(ctx, db))
	assert("Reload restored name", u.Name == "Updated Name")
}

// ---------------------------------------------------------------------------
// 5. CRUD Upsert
// ---------------------------------------------------------------------------

func testCRUDUpsert(ctx context.Context, db *sql.DB) {
	section("CRUD Upsert")
	truncateAll(db)

	// Upsert new row.
	u := &models.User{
		ID:      fake.UUID(),
		Email:   "upsert@example.com",
		Name:    "Upserted",
		IsAdmin: false,
	}
	check("Upsert new", u.Upsert(ctx, db))

	found, err := models.FindUserByPK(ctx, db, u.ID)
	check("Find after upsert new", err)
	assert("Upsert new: name correct", found.Name == "Upserted")

	// Upsert existing (updates).
	u.Name = "Upserted Again"
	u.IsAdmin = true
	check("Upsert existing", u.Upsert(ctx, db))

	found2, err := models.FindUserByPK(ctx, db, u.ID)
	check("Find after upsert existing", err)
	assert("Upsert existing: name updated", found2.Name == "Upserted Again")
	assert("Upsert existing: is_admin updated", found2.IsAdmin)

	// Verify only 1 row exists.
	count, _ := models.CountUsers(ctx, db)
	assert("Upsert: still 1 row", count == 1)
}

// ---------------------------------------------------------------------------
// 6. CRUD Delete
// ---------------------------------------------------------------------------

func testCRUDDelete(ctx context.Context, db *sql.DB) {
	section("CRUD Delete")
	truncateAll(db)

	u1 := insertTestUser(ctx, db, "del1@example.com", "Del1")
	_ = insertTestUser(ctx, db, "del2@example.com", "Del2")
	u3 := insertTestUser(ctx, db, "del3@example.com", "Del3")

	// Delete single.
	check("Delete single", u1.Delete(ctx, db))
	count, _ := models.CountUsers(ctx, db)
	assert("After delete single: count=2", count == 2)

	// DeleteAll with mod.
	n, err := models.DeleteAllUsers(ctx, db, models.UserWhere.Email.EQ("del2@example.com"))
	check("DeleteAllUsers with where", err)
	assert("DeleteAllUsers affected 1", n == 1)

	// Slice.DeleteAll.
	remaining, _ := models.AllUsers(ctx, db)
	assert("1 user remaining", len(remaining) == 1)
	assert("Remaining is del3", remaining[0].ID == u3.ID)

	n, err = remaining.DeleteAll(ctx, db)
	check("Slice.DeleteAll", err)
	assert("Slice.DeleteAll affected 1", n == 1)

	count, _ = models.CountUsers(ctx, db)
	assert("All users deleted", count == 0)
}

// ---------------------------------------------------------------------------
// 7. Query Builder
// ---------------------------------------------------------------------------

func testQueryBuilder(ctx context.Context, db *sql.DB) {
	section("Query Builder")
	truncateAll(db)

	// Set up data.
	u1 := insertTestUser(ctx, db, "qb1@example.com", "Alice")
	u2 := insertTestUser(ctx, db, "qb2@example.com", "Bob")
	_ = u2

	cat := &models.Category{Name: "Tech"}
	must(cat.Insert(ctx, db))

	p1 := &models.Post{AuthorID: u1.ID, CategoryID: sqlgen.NewNull(cat.ID), Title: "Go Tips", Body: "body1", Status: models.PostStatusPublished}
	p2 := &models.Post{AuthorID: u1.ID, Title: "SQL Tricks", Body: "body2", Status: models.PostStatusDraft}
	p3 := &models.Post{AuthorID: u2.ID, Title: "Rust Notes", Body: "body3", Status: models.PostStatusPublished}
	must(p1.Insert(ctx, db))
	must(p2.Insert(ctx, db))
	must(p3.Insert(ctx, db))

	// Where + Or.
	q := models.Posts(
		sqlgen.Where("\"title\" = ?", "Go Tips"),
		sqlgen.Or("\"title\" = ?", "SQL Tricks"),
		sqlgen.OrderBy("\"title\" ASC"),
	)
	query, args := q.BuildSelect()
	rows, err := db.QueryContext(ctx, query, args...)
	check("Where+Or query", err)
	if err == nil {
		var titles []string
		for rows.Next() {
			var p models.Post
			must(p.ScanRow(rows))
			titles = append(titles, p.Title)
		}
		rows.Close()
		assert("Where+Or result count", len(titles) == 2, "got %d", len(titles))
	}

	// WhereIn.
	posts, err := models.AllPosts(ctx, db, sqlgen.WhereIn("\"title\"", "Go Tips", "Rust Notes"), sqlgen.OrderBy("\"title\" ASC"))
	check("WhereIn", err)
	assert("WhereIn returns 2", len(posts) == 2)

	// Expr (grouped conditions).
	posts, err = models.AllPosts(ctx, db,
		sqlgen.Where("\"status\" = ?", "published"),
		sqlgen.Expr(
			sqlgen.Where("\"title\" = ?", "Go Tips"),
			sqlgen.Or("\"title\" = ?", "Rust Notes"),
		),
	)
	check("Expr grouped", err)
	assert("Expr returns 2 published", len(posts) == 2)

	// GroupBy + Having.
	q = models.Posts(
		sqlgen.Select("\"author_id\"", "COUNT(*) AS cnt"),
		sqlgen.GroupBy("\"author_id\""),
		sqlgen.Having("COUNT(*) > ?", 1),
	)
	query, args = q.BuildSelect()
	var authorID string
	var cnt int64
	err = db.QueryRowContext(ctx, query, args...).Scan(&authorID, &cnt)
	check("GroupBy+Having", err)
	assert("GroupBy+Having: u1 has 2 posts", authorID == u1.ID && cnt == 2)

	// DistinctOn.
	q = models.Posts(
		sqlgen.DistinctOn("\"author_id\""),
		sqlgen.OrderBy("\"author_id\"", "\"created_at\" DESC"),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("DistinctOn query", err)
	if err == nil {
		var distinctCount int
		for rows.Next() {
			var p models.Post
			must(p.ScanRow(rows))
			distinctCount++
		}
		rows.Close()
		assert("DistinctOn returns 2 (one per author)", distinctCount == 2)
	}

	// Join.
	q = models.Posts(
		sqlgen.Select("\"posts\".\"title\"", "\"users\".\"name\""),
		sqlgen.Join("\"users\"", "\"users\".\"id\" = \"posts\".\"author_id\""),
		sqlgen.Where("\"users\".\"name\" = ?", "Alice"),
		sqlgen.OrderBy("\"posts\".\"title\" ASC"),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("Join query", err)
	if err == nil {
		var joinResults int
		for rows.Next() {
			var title, name string
			must(rows.Scan(&title, &name))
			joinResults++
		}
		rows.Close()
		assert("Join returns Alice's 2 posts", joinResults == 2)
	}

	// LeftJoin.
	q = models.Posts(
		sqlgen.Select("\"posts\".\"title\"", "\"categories\".\"name\""),
		sqlgen.LeftJoin("\"categories\"", "\"categories\".\"id\" = \"posts\".\"category_id\""),
		sqlgen.OrderBy("\"posts\".\"title\" ASC"),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("LeftJoin query", err)
	if err == nil {
		var withCat, withoutCat int
		for rows.Next() {
			var title string
			var catName sql.NullString
			must(rows.Scan(&title, &catName))
			if catName.Valid {
				withCat++
			} else {
				withoutCat++
			}
		}
		rows.Close()
		assert("LeftJoin: 1 with category", withCat == 1)
		assert("LeftJoin: 2 without category", withoutCat == 2)
	}

	// CTE (WITH clause).
	q = models.Users(
		sqlgen.WithCTE("published_authors",
			"SELECT DISTINCT \"author_id\" FROM \"posts\" WHERE \"status\" = ?", "published"),
		sqlgen.Where("\"id\" IN (SELECT \"author_id\" FROM \"published_authors\")"),
		sqlgen.OrderBy("\"name\" ASC"),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("CTE query", err)
	if err == nil {
		var cteCount int
		for rows.Next() {
			var u models.User
			must(u.ScanRow(rows))
			cteCount++
		}
		rows.Close()
		assert("CTE returns 2 published authors", cteCount == 2)
	}

	// Subquery (WhereSubquery).
	subQ := models.Posts(sqlgen.Select("\"author_id\""), sqlgen.Where("\"status\" = ?", "published"))
	usersQ := models.Users(sqlgen.WhereSubquery("\"id\"", "IN", subQ))
	query, args = usersQ.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("WhereSubquery", err)
	if err == nil {
		var subCount int
		for rows.Next() {
			var u models.User
			must(u.ScanRow(rows))
			subCount++
		}
		rows.Close()
		assert("WhereSubquery returns 2", subCount == 2)
	}

	// WhereExists.
	usersQ = models.Users(
		sqlgen.WhereExists(
			models.Posts(
				sqlgen.Select("1"),
				sqlgen.Where("\"posts\".\"author_id\" = \"users\".\"id\""),
				sqlgen.Where("\"posts\".\"status\" = ?", "draft"),
			),
		),
	)
	query, args = usersQ.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("WhereExists", err)
	if err == nil {
		var existsCount int
		for rows.Next() {
			var u models.User
			must(u.ScanRow(rows))
			existsCount++
		}
		rows.Close()
		assert("WhereExists returns 1 (Alice has draft)", existsCount == 1)
	}

	// WhereNotExists.
	usersQ = models.Users(
		sqlgen.WhereNotExists(
			models.Posts(
				sqlgen.Select("1"),
				sqlgen.Where("\"posts\".\"author_id\" = \"users\".\"id\""),
				sqlgen.Where("\"posts\".\"status\" = ?", "draft"),
			),
		),
	)
	query, args = usersQ.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("WhereNotExists", err)
	if err == nil {
		var notExistsCount int
		for rows.Next() {
			var u models.User
			must(u.ScanRow(rows))
			notExistsCount++
		}
		rows.Close()
		assert("WhereNotExists returns 1 (Bob has no draft)", notExistsCount == 1)
	}

	// Union.
	q1 := models.Posts(sqlgen.Select("\"title\""), sqlgen.Where("\"status\" = ?", "published"))
	q2 := models.Posts(sqlgen.Select("\"title\""), sqlgen.Where("\"status\" = ?", "draft"))
	unionQ := sqlgen.NewQuery(sqlgen.PostgresDialect{}, "posts",
		sqlgen.Select("\"title\""),
		sqlgen.Where("\"status\" = ?", "published"),
		sqlgen.Union(q2),
	)
	query, args = unionQ.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("Union query", err)
	if err == nil {
		var unionCount int
		for rows.Next() {
			var t string
			must(rows.Scan(&t))
			unionCount++
		}
		rows.Close()
		assert("Union returns 3 distinct titles", unionCount == 3)
	}
	_ = q1

	// Window function.
	q = models.Posts(
		sqlgen.Select("\"title\""),
		sqlgen.SelectWithWindow("ROW_NUMBER()",
			sqlgen.NewWindowDef().PartitionBy("\"author_id\"").OrderBy("\"created_at\" DESC"),
			"row_num",
		),
		sqlgen.OrderBy("\"author_id\"", "row_num"),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("Window function query", err)
	if err == nil {
		var windowCount int
		for rows.Next() {
			var title string
			var rowNum int
			must(rows.Scan(&title, &rowNum))
			windowCount++
		}
		rows.Close()
		assert("Window returns 3 rows", windowCount == 3)
	}

	// Row locking (just verify it builds and executes).
	q = models.Users(
		sqlgen.ForUpdate(),
		sqlgen.Nowait(),
		sqlgen.Limit(1),
	)
	query, args = q.BuildSelect()
	assert("ForUpdate+Nowait SQL contains FOR UPDATE",
		strings.Contains(query, "FOR UPDATE"))
	assert("ForUpdate+Nowait SQL contains NOWAIT",
		strings.Contains(query, "NOWAIT"))

	q = models.Users(
		sqlgen.ForShare(),
		sqlgen.SkipLocked(),
		sqlgen.Limit(1),
	)
	query, args = q.BuildSelect()
	assert("ForShare+SkipLocked SQL", strings.Contains(query, "FOR SHARE") && strings.Contains(query, "SKIP LOCKED"))

	// RawSQL.
	raw := sqlgen.RawSQL("SELECT COUNT(*) FROM \"users\"")
	var rawCount int64
	err = raw.QueryRow(ctx, db).Scan(&rawCount)
	check("RawSQL", err)
	assert("RawSQL count == 2", rawCount == 2)

	// FromSubquery.
	innerQ := models.Posts(
		sqlgen.Select("\"author_id\"", "COUNT(*) AS post_count"),
		sqlgen.GroupBy("\"author_id\""),
	)
	outerQ := sqlgen.NewQuery(sqlgen.PostgresDialect{}, "",
		sqlgen.FromSubquery("sub", innerQ),
		sqlgen.Select("sub.post_count"),
		sqlgen.OrderBy("sub.post_count DESC"),
		sqlgen.Limit(1),
	)
	query, args = outerQ.BuildSelect()
	var topCount int64
	err = db.QueryRowContext(ctx, query, args...).Scan(&topCount)
	check("FromSubquery", err)
	assert("FromSubquery top count == 2", topCount == 2)

	// Distinct.
	q = models.Posts(sqlgen.Select("\"status\""), sqlgen.Distinct(), sqlgen.OrderBy("\"status\" ASC"))
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("Distinct query", err)
	if err == nil {
		var distinctStatuses int
		for rows.Next() {
			var s string
			must(rows.Scan(&s))
			distinctStatuses++
		}
		rows.Close()
		assert("Distinct statuses == 2", distinctStatuses == 2)
	}

	// Offset.
	allUsers, _ := models.AllUsers(ctx, db, sqlgen.OrderBy("\"email\" ASC"))
	offsetUsers, err := models.AllUsers(ctx, db, sqlgen.OrderBy("\"email\" ASC"), sqlgen.Offset(1))
	check("Offset query", err)
	if len(allUsers) > 1 {
		assert("Offset skips first", len(offsetUsers) == len(allUsers)-1)
	}

	// BuildInsert (standalone).
	query, args = sqlgen.BuildInsert(sqlgen.PostgresDialect{}, "users",
		[]string{"email", "name", "is_admin"},
		[]any{"build_insert@example.com", "BuildInsert", false},
		[]string{"id"},
	)
	assert("BuildInsert contains RETURNING", strings.Contains(query, "RETURNING"))
	var insertedID string
	err = db.QueryRowContext(ctx, query, args...).Scan(&insertedID)
	check("BuildInsert exec", err)
	assert("BuildInsert returned ID", insertedID != "")

	// BuildBatchInsert.
	batchRows := [][]any{
		{"batch1@example.com", "Batch1", false},
		{"batch2@example.com", "Batch2", true},
	}
	query, args = sqlgen.BuildBatchInsert(sqlgen.PostgresDialect{}, "users",
		[]string{"email", "name", "is_admin"},
		batchRows,
		[]string{"id"},
	)
	rows, err = db.QueryContext(ctx, query, args...)
	check("BuildBatchInsert exec", err)
	if err == nil {
		var batchCount int
		for rows.Next() {
			var id string
			must(rows.Scan(&id))
			batchCount++
		}
		rows.Close()
		assert("BuildBatchInsert inserted 2", batchCount == 2)
	}

	// BuildUpdate.
	query, args = sqlgen.BuildUpdate(sqlgen.PostgresDialect{}, "users",
		[]string{"name"}, []any{"BuildUpdated"},
		[]string{"\"email\" = ?"}, []any{"build_insert@example.com"},
	)
	_, err = db.ExecContext(ctx, query, args...)
	check("BuildUpdate exec", err)

	// BuildDelete.
	query, args = sqlgen.BuildDelete(sqlgen.PostgresDialect{}, "users",
		[]string{"\"email\" = ?"}, []any{"build_insert@example.com"},
	)
	_, err = db.ExecContext(ctx, query, args...)
	check("BuildDelete exec", err)

	// BuildUpsert.
	upsertID := fake.UUID()
	query, args = sqlgen.BuildUpsert(sqlgen.PostgresDialect{}, "users",
		[]string{"id", "email", "name", "is_admin"},
		[]any{upsertID, "buildupsert@example.com", "BuildUpsert", false},
		[]string{"id"},
		[]string{"email", "name", "is_admin"},
		[]string{"id"},
	)
	var returnedID string
	err = db.QueryRowContext(ctx, query, args...).Scan(&returnedID)
	check("BuildUpsert exec", err)
	assert("BuildUpsert returned same ID", returnedID == upsertID)

	// BuildInsertSelect.
	query, args = sqlgen.BuildInsertSelect(sqlgen.PostgresDialect{}, "users",
		[]string{"email", "name", "is_admin"},
		sqlgen.NewQuery(sqlgen.PostgresDialect{}, "users",
			sqlgen.Select("\"email\" || '_copy'", "\"name\"", "\"is_admin\""),
			sqlgen.Limit(1),
		),
		nil,
	)
	_, err = db.ExecContext(ctx, query, args...)
	check("BuildInsertSelect exec", err)

	// Intersect.
	q = sqlgen.NewQuery(sqlgen.PostgresDialect{}, "posts",
		sqlgen.Select("\"title\""),
		sqlgen.Where("\"status\" = ?", "published"),
		sqlgen.Intersect(
			sqlgen.NewQuery(sqlgen.PostgresDialect{}, "posts",
				sqlgen.Select("\"title\""),
				sqlgen.Where("\"author_id\" = ?", u1.ID),
			),
		),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("Intersect query", err)
	if err == nil {
		var isectCount int
		for rows.Next() {
			var t string
			must(rows.Scan(&t))
			isectCount++
		}
		rows.Close()
		assert("Intersect returns 1 (Go Tips)", isectCount == 1)
	}

	// Except.
	q = sqlgen.NewQuery(sqlgen.PostgresDialect{}, "posts",
		sqlgen.Select("\"title\""),
		sqlgen.Where("\"status\" = ?", "published"),
		sqlgen.Except(
			sqlgen.NewQuery(sqlgen.PostgresDialect{}, "posts",
				sqlgen.Select("\"title\""),
				sqlgen.Where("\"author_id\" = ?", u1.ID),
			),
		),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("Except query", err)
	if err == nil {
		var exceptCount int
		for rows.Next() {
			var t string
			must(rows.Scan(&t))
			exceptCount++
		}
		rows.Close()
		assert("Except returns 1 (Rust Notes)", exceptCount == 1)
	}

	// WithRecursiveCTE.
	// Set up parent-child categories.
	child := &models.Category{Name: "Go", ParentID: sqlgen.NewNull(cat.ID)}
	must(child.Insert(ctx, db))
	grandchild := &models.Category{Name: "Generics", ParentID: sqlgen.NewNull(child.ID)}
	must(grandchild.Insert(ctx, db))

	q = sqlgen.NewQuery(sqlgen.PostgresDialect{}, "cat_tree",
		sqlgen.WithRecursiveCTE("cat_tree",
			"SELECT \"id\", \"name\", \"parent_id\" FROM \"categories\" WHERE \"id\" = ? "+
				"UNION ALL "+
				"SELECT c.\"id\", c.\"name\", c.\"parent_id\" FROM \"categories\" c "+
				"JOIN \"cat_tree\" ct ON c.\"parent_id\" = ct.\"id\"",
			cat.ID,
		),
		sqlgen.Select("\"name\""),
		sqlgen.OrderBy("\"id\" ASC"),
	)
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("RecursiveCTE query", err)
	if err == nil {
		var names []string
		for rows.Next() {
			var n string
			must(rows.Scan(&n))
			names = append(names, n)
		}
		rows.Close()
		assert("RecursiveCTE returns 3 categories", len(names) == 3, "got %v", names)
	}

	// CrossJoin (just verify it builds).
	q = sqlgen.NewQuery(sqlgen.PostgresDialect{}, "users",
		sqlgen.Select("\"users\".\"name\"", "\"categories\".\"name\""),
		sqlgen.CrossJoin("\"categories\""),
		sqlgen.Limit(5),
	)
	query, args = q.BuildSelect()
	assert("CrossJoin SQL contains CROSS JOIN", strings.Contains(query, "CROSS JOIN"))
	rows, err = db.QueryContext(ctx, query, args...)
	check("CrossJoin exec", err)
	if err == nil {
		var crossCount int
		for rows.Next() {
			var uName, cName string
			must(rows.Scan(&uName, &cName))
			crossCount++
		}
		rows.Close()
		assert("CrossJoin returns rows", crossCount > 0)
	}

	// RightJoin + FullJoin (verify SQL builds correctly).
	q = models.Posts(
		sqlgen.Select("\"posts\".\"title\"", "\"categories\".\"name\""),
		sqlgen.RightJoin("\"categories\"", "\"categories\".\"id\" = \"posts\".\"category_id\""),
	)
	query, _ = q.BuildSelect()
	assert("RightJoin SQL", strings.Contains(query, "RIGHT JOIN"))

	q = models.Posts(
		sqlgen.Select("\"posts\".\"title\"", "\"categories\".\"name\""),
		sqlgen.FullJoin("\"categories\"", "\"categories\".\"id\" = \"posts\".\"category_id\""),
	)
	query, _ = q.BuildSelect()
	assert("FullJoin SQL", strings.Contains(query, "FULL JOIN"))
}

// ---------------------------------------------------------------------------
// 8. Type-Safe Where
// ---------------------------------------------------------------------------

func testTypeSafeWhere(ctx context.Context, db *sql.DB) {
	section("Type-Safe Where")
	truncateAll(db)

	u1 := insertTestUser(ctx, db, "where1@example.com", "Alice")
	u2 := insertTestUser(ctx, db, "where2@example.com", "Bob")

	// Set bio on u1 only.
	u1.Bio.Set("Has a bio")
	must(u1.Update(ctx, db))

	// EQ.
	users, err := models.AllUsers(ctx, db, models.UserWhere.Name.EQ("Alice"))
	check("Where.Name.EQ", err)
	assert("Where.Name.EQ returns 1", len(users) == 1 && users[0].Name == "Alice")

	// NEQ.
	users, err = models.AllUsers(ctx, db, models.UserWhere.Name.NEQ("Alice"))
	check("Where.Name.NEQ", err)
	assert("Where.Name.NEQ returns Bob", len(users) == 1 && users[0].Name == "Bob")

	// LT/GT.
	users, err = models.AllUsers(ctx, db, models.UserWhere.Email.LT("where2@example.com"))
	check("Where.Email.LT", err)
	assert("Where.Email.LT returns 1", len(users) == 1)

	users, err = models.AllUsers(ctx, db, models.UserWhere.Email.GT("where1@example.com"))
	check("Where.Email.GT", err)
	assert("Where.Email.GT returns 1", len(users) == 1)

	// IN.
	users, err = models.AllUsers(ctx, db, models.UserWhere.Name.IN("Alice", "Bob"))
	check("Where.Name.IN", err)
	assert("Where.Name.IN returns 2", len(users) == 2)

	// IsNull/IsNotNull.
	users, err = models.AllUsers(ctx, db, models.UserWhere.Bio.IsNull())
	check("Where.Bio.IsNull", err)
	assert("Where.Bio.IsNull returns 1 (Bob)", len(users) == 1 && users[0].Name == "Bob")

	users, err = models.AllUsers(ctx, db, models.UserWhere.Bio.IsNotNull())
	check("Where.Bio.IsNotNull", err)
	assert("Where.Bio.IsNotNull returns 1 (Alice)", len(users) == 1 && users[0].Name == "Alice")

	// LTE/GTE.
	users, err = models.AllUsers(ctx, db, models.UserWhere.Email.LTE("where1@example.com"))
	check("Where.Email.LTE", err)
	assert("Where.Email.LTE returns 1", len(users) == 1)

	users, err = models.AllUsers(ctx, db, models.UserWhere.Email.GTE("where2@example.com"))
	check("Where.Email.GTE", err)
	assert("Where.Email.GTE returns 1", len(users) == 1)

	// IsAdmin filter (bool type).
	u1.IsAdmin = true
	must(u1.Update(ctx, db))
	users, err = models.AllUsers(ctx, db, models.UserWhere.IsAdmin.EQ(true))
	check("Where.IsAdmin.EQ(true)", err)
	assert("Where.IsAdmin.EQ(true) returns 1", len(users) == 1)

	// Use u2 to verify NEQ returns it.
	assert("u2 exists", u2.ID != "")
}

// ---------------------------------------------------------------------------
// 9. Relationships
// ---------------------------------------------------------------------------

func testRelationships(ctx context.Context, db *sql.DB) {
	section("Relationships")
	truncateAll(db)

	u := insertTestUser(ctx, db, "rel@example.com", "RelUser")

	cat1 := &models.Category{Name: "Parent Cat"}
	must(cat1.Insert(ctx, db))
	cat2 := &models.Category{Name: "Child Cat"}
	must(cat2.Insert(ctx, db))

	p := &models.Post{AuthorID: u.ID, Title: "Rel Post", Body: "body", Status: models.PostStatusDraft}
	must(p.Insert(ctx, db))

	tag1 := &models.Tag{Name: "tag-a"}
	tag2 := &models.Tag{Name: "tag-b"}
	tag3 := &models.Tag{Name: "tag-c"}
	must(tag1.Insert(ctx, db))
	must(tag2.Insert(ctx, db))
	must(tag3.Insert(ctx, db))

	prof := &models.Profile{UserID: u.ID, AvatarURL: sqlgen.NewNull("https://example.com/avatar.png")}
	must(prof.Insert(ctx, db))

	comment := &models.Comment{AuthorID: u.ID, CommentableType: "Post", CommentableID: p.ID, Kind: models.CommentKindNote, Body: "nice"}
	must(comment.Insert(ctx, db))

	// SetUser on Post.
	u2 := insertTestUser(ctx, db, "rel2@example.com", "RelUser2")
	check("Post.SetUser", p.SetUser(ctx, db, u2))
	reloaded, _ := models.FindPostByPK(ctx, db, p.ID)
	assert("Post.SetUser updated FK", reloaded.AuthorID == u2.ID)

	// Switch back.
	must(p.SetUser(ctx, db, u))

	// AddTags.
	check("Post.AddTags", p.AddTags(ctx, db, tag1, tag2))
	assert("Post.R.Tags after AddTags", p.R != nil && len(p.R.Tags) == 2)

	// RemoveTags.
	check("Post.RemoveTags", p.RemoveTags(ctx, db, tag1))
	assert("Post.R.Tags after RemoveTags", len(p.R.Tags) == 1 && p.R.Tags[0].Name == "tag-b")

	// SetTags (replace all).
	check("Post.SetTags", p.SetTags(ctx, db, tag2, tag3))
	assert("Post.R.Tags after SetTags", len(p.R.Tags) == 2)

	// SetCategory.
	check("Post.SetCategory", p.SetCategory(ctx, db, cat1))
	assert("Post.R.Category set", p.R.Category != nil && p.R.Category.Name == "Parent Cat")

	// RemoveCategory.
	check("Post.RemoveCategory", p.RemoveCategory(ctx, db))
	assert("Post.R.Category nil after remove", p.R.Category == nil)

	// SetParent on Category.
	check("Category.SetParent", cat2.SetParent(ctx, db, cat1))
	assert("Category.R.Parent set", cat2.R != nil && cat2.R.Parent.ID == cat1.ID)

	// RemoveParent.
	check("Category.RemoveParent", cat2.RemoveParent(ctx, db))
	assert("Category.R.Parent nil", cat2.R == nil || cat2.R.Parent == nil)

	// User.SetProfile.
	check("User.SetProfile", u.SetProfile(ctx, db, prof))
	assert("User.R.Profile set", u.R != nil && u.R.Profile.ID == prof.ID)

	// User.AddPosts.
	check("User.AddPosts", u.AddPosts(ctx, db, p))
	assert("User.R.Posts populated", u.R != nil && len(u.R.Posts) > 0)

	// User.AddComments.
	check("User.AddComments", u.AddComments(ctx, db, comment))
	assert("User.R.Comments populated", u.R != nil && len(u.R.Comments) > 0)
}

// ---------------------------------------------------------------------------
// 10. Eager Loading
// ---------------------------------------------------------------------------

func testEagerLoading(ctx context.Context, db *sql.DB) {
	section("Eager Loading")
	truncateAll(db)

	u := insertTestUser(ctx, db, "eager@example.com", "EagerUser")
	p := &models.Post{AuthorID: u.ID, Title: "Eager Post", Body: "body", Status: models.PostStatusDraft}
	must(p.Insert(ctx, db))

	tag := &models.Tag{Name: "eager-tag"}
	must(tag.Insert(ctx, db))
	must(p.AddTags(ctx, db, tag))

	comment := &models.Comment{AuthorID: u.ID, CommentableType: "Post", CommentableID: p.ID, Kind: models.CommentKindReview, Body: "loaded!"}
	must(comment.Insert(ctx, db))

	prof := &models.Profile{UserID: u.ID, Website: sqlgen.NewNull("https://eager.dev")}
	must(prof.Insert(ctx, db))

	// Load("Posts") on users.
	users := models.UserSlice{u}
	check("LoadRelations Posts", users.LoadRelations(ctx, db, sqlgen.Load("Posts")))
	assert("User.R.Posts loaded", u.R != nil && len(u.R.Posts) == 1)

	// Load("Profile") on users.
	u.R = nil
	check("LoadRelations Profile", users.LoadRelations(ctx, db, sqlgen.Load("Profile")))
	assert("User.R.Profile loaded", u.R != nil && u.R.Profile != nil)

	// Load("Tags") on posts.
	posts := models.PostSlice{p}
	p.R = nil
	check("LoadRelations Tags", posts.LoadRelations(ctx, db, sqlgen.Load("Tags")))
	assert("Post.R.Tags loaded", p.R != nil && len(p.R.Tags) == 1)

	// Load("User") on posts.
	p.R = nil
	check("LoadRelations User", posts.LoadRelations(ctx, db, sqlgen.Load("User")))
	assert("Post.R.User loaded", p.R != nil && p.R.User != nil && p.R.User.Email == "eager@example.com")

	// Nested: Load("Posts.Tags") on users.
	u.R = nil
	check("LoadRelations Posts.Tags", users.LoadRelations(ctx, db, sqlgen.Load("Posts.Tags")))
	assert("Nested: user has posts", u.R != nil && len(u.R.Posts) == 1)
	assert("Nested: post has tags", u.R.Posts[0].R != nil && len(u.R.Posts[0].R.Tags) == 1)

	// LoadCountRelations.
	u.R = nil
	check("LoadCountRelations Posts", users.LoadCountRelations(ctx, db, "Posts"))
	assert("PostsCount loaded", u.R != nil && u.R.PostsCount != nil && *u.R.PostsCount == 1)

	check("LoadCountRelations Comments", users.LoadCountRelations(ctx, db, "Comments"))
	assert("CommentsCount loaded", u.R.CommentsCount != nil && *u.R.CommentsCount == 1)
}

// ---------------------------------------------------------------------------
// 11. Preloading (LEFT JOIN)
// ---------------------------------------------------------------------------

func testPreloading(ctx context.Context, db *sql.DB) {
	section("Preloading")
	truncateAll(db)

	u := insertTestUser(ctx, db, "preload@example.com", "PreloadUser")
	p := &models.Post{AuthorID: u.ID, Title: "Preload Post", Body: "body", Status: models.PostStatusDraft}
	must(p.Insert(ctx, db))

	cat := &models.Category{Name: "Preload Cat"}
	must(cat.Insert(ctx, db))
	must(p.SetCategory(ctx, db, cat))

	prof := &models.Profile{UserID: u.ID, AvatarURL: sqlgen.NewNull("https://preload.dev/a.png")}
	must(prof.Insert(ctx, db))

	// Preload PostPreloadUser.
	posts, err := models.AllPosts(ctx, db, sqlgen.Preload(models.PostPreloadUser))
	check("Preload PostPreloadUser", err)
	assert("Preloaded user populated", len(posts) == 1 && posts[0].R != nil && posts[0].R.User != nil)
	assert("Preloaded user email", posts[0].R.User.Email == "preload@example.com")

	// Preload PostPreloadCategory.
	posts, err = models.AllPosts(ctx, db, sqlgen.Preload(models.PostPreloadCategory))
	check("Preload PostPreloadCategory", err)
	assert("Preloaded category populated", posts[0].R != nil && posts[0].R.Category != nil)
	assert("Preloaded category name", posts[0].R.Category.Name == "Preload Cat")

	// Preload UserPreloadProfile.
	users, err := models.AllUsers(ctx, db, sqlgen.Preload(models.UserPreloadProfile))
	check("Preload UserPreloadProfile", err)
	assert("Preloaded profile populated", len(users) == 1 && users[0].R != nil && users[0].R.Profile != nil)
	assert("Preloaded profile avatar", users[0].R.Profile.AvatarURL.Valid)

	// Preload with no match (user without profile).
	u2 := insertTestUser(ctx, db, "noprofile@example.com", "NoProfile")
	_ = u2
	users, err = models.AllUsers(ctx, db,
		sqlgen.Preload(models.UserPreloadProfile),
		sqlgen.OrderBy("\"email\" ASC"),
	)
	check("Preload with null LEFT JOIN", err)
	assert("Preload: 2 users", len(users) == 2)
	// noprofile@example.com comes first alphabetically.
	assert("Preload: first user has no profile", users[0].R == nil || users[0].R.Profile == nil)
	assert("Preload: second user has profile", users[1].R != nil && users[1].R.Profile != nil)
}

// ---------------------------------------------------------------------------
// 12. Hooks
// ---------------------------------------------------------------------------

func testHooks(ctx context.Context, db *sql.DB) {
	section("Hooks")
	truncateAll(db)

	var hookLog []string

	models.AddUserHook(sqlgen.BeforeInsert, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "BeforeInsert")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.AfterInsert, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "AfterInsert")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.BeforeUpdate, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "BeforeUpdate")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.AfterUpdate, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "AfterUpdate")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.BeforeDelete, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "BeforeDelete")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.AfterDelete, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "AfterDelete")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.BeforeUpsert, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "BeforeUpsert")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.AfterUpsert, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "AfterUpsert")
		return ctx, nil
	})
	models.AddUserHook(sqlgen.AfterSelect, func(ctx context.Context, exec sqlgen.Executor, model *models.User) (context.Context, error) {
		hookLog = append(hookLog, "AfterSelect")
		return ctx, nil
	})

	// Insert triggers hooks.
	u := &models.User{Email: "hooks@example.com", Name: "HookUser"}
	must(u.Insert(ctx, db))
	assert("BeforeInsert fired", contains(hookLog, "BeforeInsert"))
	assert("AfterInsert fired", contains(hookLog, "AfterInsert"))

	// Update.
	hookLog = nil
	u.Name = "UpdatedHook"
	must(u.Update(ctx, db))
	assert("BeforeUpdate fired", contains(hookLog, "BeforeUpdate"))
	assert("AfterUpdate fired", contains(hookLog, "AfterUpdate"))

	// Upsert.
	hookLog = nil
	u.Name = "UpsertedHook"
	must(u.Upsert(ctx, db))
	assert("BeforeUpsert fired", contains(hookLog, "BeforeUpsert"))
	assert("AfterUpsert fired", contains(hookLog, "AfterUpsert"))

	// Delete.
	hookLog = nil
	must(u.Delete(ctx, db))
	assert("BeforeDelete fired", contains(hookLog, "BeforeDelete"))
	assert("AfterDelete fired", contains(hookLog, "AfterDelete"))

	// SkipHooks.
	hookLog = nil
	skipCtx := sqlgen.SkipHooks(ctx)
	assert("ShouldSkipHooks true", sqlgen.ShouldSkipHooks(skipCtx))
	u2 := &models.User{Email: "skip@example.com", Name: "SkipUser"}
	must(u2.Insert(skipCtx, db))
	assert("No hooks fired with SkipHooks", len(hookLog) == 0, "got %v", hookLog)
}

// ---------------------------------------------------------------------------
// 13. Each/Cursor
// ---------------------------------------------------------------------------

func testEachCursor(ctx context.Context, db *sql.DB) {
	section("Each/Cursor")
	truncateAll(db)

	for i := 0; i < 5; i++ {
		insertTestUser(ctx, db, fmt.Sprintf("each%d@example.com", i), fmt.Sprintf("Each%d", i))
	}

	// EachUser.
	var eachNames []string
	err := models.EachUser(ctx, db, func(u *models.User) error {
		eachNames = append(eachNames, u.Name)
		return nil
	}, sqlgen.OrderBy("\"name\" ASC"))
	check("EachUser", err)
	assert("EachUser iterated 5", len(eachNames) == 5)

	// UserCursor.
	cursor, err := models.UserCursor(ctx, db, sqlgen.OrderBy("\"name\" ASC"), sqlgen.Limit(3))
	check("UserCursor create", err)
	if err == nil {
		var cursorNames []string
		for {
			u, ok := cursor.Next()
			if !ok {
				break
			}
			cursorNames = append(cursorNames, u.Name)
		}
		check("UserCursor.Err", cursor.Err())
		check("UserCursor.Close", cursor.Close())
		assert("UserCursor iterated 3", len(cursorNames) == 3)
	}
}

// ---------------------------------------------------------------------------
// 14. Null[T]
// ---------------------------------------------------------------------------

func testNullType(ctx context.Context, db *sql.DB) {
	section("Null[T]")
	truncateAll(db)

	// NewNull.
	n := sqlgen.NewNull("hello")
	assert("NewNull valid", n.Valid)
	assert("NewNull val", n.Val == "hello")

	// NullVal.
	nv := sqlgen.NullVal[string]()
	assert("NullVal not valid", !nv.Valid)

	// Set.
	nv.Set("world")
	assert("Set makes valid", nv.Valid && nv.Val == "world")

	// Clear.
	nv.Clear()
	assert("Clear makes invalid", !nv.Valid)

	// Ptr.
	n2 := sqlgen.NewNull(42)
	ptr := n2.Ptr()
	assert("Ptr non-nil", ptr != nil && *ptr == 42)
	nv2 := sqlgen.NullVal[int]()
	assert("Ptr nil for null", nv2.Ptr() == nil)

	// FromPtr.
	val := 99
	fromPtr := sqlgen.FromPtr(&val)
	assert("FromPtr valid", fromPtr.Valid && fromPtr.Val == 99)
	fromNil := sqlgen.FromPtr[int](nil)
	assert("FromPtr(nil) null", !fromNil.Valid)

	// JSON round-trip.
	n3 := sqlgen.NewNull("json-val")
	data, err := json.Marshal(n3)
	check("Null JSON marshal", err)
	assert("Null JSON marshal value", string(data) == `"json-val"`)

	var n4 sqlgen.Null[string]
	check("Null JSON unmarshal", json.Unmarshal(data, &n4))
	assert("Null JSON unmarshal valid", n4.Valid && n4.Val == "json-val")

	// JSON null.
	var n5 sqlgen.Null[string]
	check("Null JSON unmarshal null", json.Unmarshal([]byte("null"), &n5))
	assert("Null JSON unmarshal null result", !n5.Valid)

	nullData, _ := json.Marshal(n5)
	assert("Null JSON marshal null", string(nullData) == "null")

	// DB round-trip.
	u := &models.User{
		Email:   "null_test@example.com",
		Name:    "NullTest",
		Bio:     sqlgen.NewNull("a bio"),
		Age:     sqlgen.NewNull(int32(25)),
		Score:   sqlgen.NewNull("99.50"),
		IsAdmin: false,
	}
	must(u.Insert(ctx, db))

	found, _ := models.FindUserByPK(ctx, db, u.ID)
	assert("DB round-trip Bio", found.Bio.Valid && found.Bio.Val == "a bio")
	assert("DB round-trip Age", found.Age.Valid && found.Age.Val == 25)
	assert("DB round-trip Score", found.Score.Valid && found.Score.Val == "99.50")

	// DB null round-trip.
	u2 := &models.User{Email: "null_test2@example.com", Name: "NullTest2"}
	must(u2.Insert(ctx, db))
	found2, _ := models.FindUserByPK(ctx, db, u2.ID)
	assert("DB null round-trip Bio", !found2.Bio.Valid)
	assert("DB null round-trip Age", !found2.Age.Valid)

	// String method.
	assert("Null.String non-null", sqlgen.NewNull("test").String() == "test")
	assert("Null.String null", sqlgen.NullVal[string]().String() == "NULL")
}

// ---------------------------------------------------------------------------
// 15. CachedExecutor
// ---------------------------------------------------------------------------

func testCachedExecutor(ctx context.Context, db *sql.DB) {
	section("CachedExecutor")
	truncateAll(db)

	ce := sqlgen.NewCachedExecutor(db)

	insertTestUser(ctx, db, "cached@example.com", "CachedUser")

	// Query through cached executor.
	u, err := models.FindUserByPK(ctx, ce, "00000000-0000-0000-0000-000000000000")
	_ = u
	// This will return ErrNoRows, but the statement should still be cached.
	_ = err

	u2, err := models.AllUsers(ctx, ce)
	check("CachedExecutor AllUsers", err)
	assert("CachedExecutor returned users", len(u2) > 0)

	assert("CachedExecutor.Len > 0", ce.Len() > 0, "got %d", ce.Len())
	initialLen := ce.Len()

	// Same query again should reuse.
	_, _ = models.AllUsers(ctx, ce)
	assert("CachedExecutor.Len stable", ce.Len() == initialLen)

	// Different query.
	_, _ = models.CountUsers(ctx, ce)
	assert("CachedExecutor.Len grew", ce.Len() > initialLen)

	check("CachedExecutor.Close", ce.Close())
}

// ---------------------------------------------------------------------------
// 16. Error Handling
// ---------------------------------------------------------------------------

func testErrorHandling(ctx context.Context, db *sql.DB) {
	section("Error Handling")
	truncateAll(db)

	u := insertTestUser(ctx, db, "err@example.com", "ErrUser")

	// Unique violation.
	u2 := &models.User{Email: "err@example.com", Name: "Dupe"}
	err := u2.Insert(ctx, db)
	assert("Unique violation detected", err != nil)
	assert("IsUniqueViolation", sqlgen.IsUniqueViolation(err, "users_email_key"))

	// Foreign key violation.
	p := &models.Post{AuthorID: "00000000-0000-0000-0000-000000000000", Title: "Bad", Body: "body", Status: models.PostStatusDraft}
	err = p.Insert(ctx, db)
	assert("FK violation detected", err != nil)
	assert("IsForeignKeyViolation", sqlgen.IsForeignKeyViolation(err, "posts_author_id_fkey"))

	// Not null violation.
	// Postgres NOT NULL violations have an empty ConstraintName (the column is in ColumnName).
	_, err = db.ExecContext(ctx, "INSERT INTO users (email, name, is_admin) VALUES (NULL, 'test', false)")
	assert("Not null violation detected", err != nil)
	assert("IsNotNullViolation", sqlgen.IsNotNullViolation(err, ""))

	// ErrNoRows.
	_, err = models.FindUserByPK(ctx, db, "00000000-0000-0000-0000-000000000000")
	assert("ErrNoRows", errors.Is(err, sqlgen.ErrNoRows))

	_ = u
}

// ---------------------------------------------------------------------------
// 17. Column Filtering
// ---------------------------------------------------------------------------

func testColumnFiltering(ctx context.Context, db *sql.DB) {
	section("Column Filtering")
	truncateAll(db)

	// Insert with Whitelist.
	u := &models.User{
		Email:   "whitelist@example.com",
		Name:    "Whitelist",
		Bio:     sqlgen.NewNull("should be ignored"),
		IsAdmin: true,
	}
	check("Insert with Whitelist", u.Insert(ctx, db, sqlgen.Whitelist("email", "name")))

	found, _ := models.FindUserByPK(ctx, db, u.ID)
	assert("Whitelist: email set", found.Email == "whitelist@example.com")
	assert("Whitelist: bio not set (default null)", !found.Bio.Valid)
	assert("Whitelist: is_admin default false", !found.IsAdmin)

	// Update with Blacklist.
	u.Name = "Updated"
	u.Bio.Set("new bio")
	u.IsAdmin = true
	check("Update with Blacklist", u.Update(ctx, db, sqlgen.Blacklist("is_admin")))

	found, _ = models.FindUserByPK(ctx, db, u.ID)
	assert("Blacklist: name updated", found.Name == "Updated")
	assert("Blacklist: bio updated", found.Bio.Valid && found.Bio.Val == "new bio")
	assert("Blacklist: is_admin unchanged", !found.IsAdmin)
}

// ---------------------------------------------------------------------------
// 18. Factories
// ---------------------------------------------------------------------------

func testFactories(ctx context.Context, db *sql.DB) {
	section("Factories")
	truncateAll(db)

	// NewUser with no mods.
	u := models.NewUser()
	assert("NewUser has email", u.Email != "")
	assert("NewUser has name", u.Name != "")
	assert("NewUser has ID", u.ID != "")

	// NewUser with mod.
	u2 := models.NewUser(func(u *models.User) {
		u.Email = "factory@example.com"
		u.Name = "FactoryUser"
	})
	assert("NewUser with mod: email", u2.Email == "factory@example.com")

	// InsertUser.
	inserted, err := models.InsertUser(ctx, db, func(u *models.User) {
		u.Email = "inserted_factory@example.com"
		u.Name = "InsertedFactory"
	})
	check("InsertUser factory", err)
	assert("InsertUser ID populated", inserted.ID != "")
	assert("InsertUser email", inserted.Email == "inserted_factory@example.com")

	// InsertPost (needs valid author).
	post, err := models.InsertPost(ctx, db, func(p *models.Post) {
		p.AuthorID = inserted.ID
		p.CategoryID.Clear()
		p.Title = "Factory Post"
	})
	check("InsertPost factory", err)
	assert("InsertPost ID populated", post.ID != "")

	// InsertTag.
	tag, err := models.InsertTag(ctx, db)
	check("InsertTag factory", err)
	assert("InsertTag ID populated", tag.ID != 0)

	// InsertCategory (no parent).
	cat, err := models.InsertCategory(ctx, db, func(c *models.Category) {
		c.ParentID.Clear()
	})
	check("InsertCategory factory", err)
	assert("InsertCategory ID populated", cat.ID != 0)

	// InsertProfile.
	prof, err := models.InsertProfile(ctx, db, func(p *models.Profile) {
		p.UserID = inserted.ID
	})
	check("InsertProfile factory", err)
	assert("InsertProfile ID populated", prof.ID != "")

	// InsertComment.
	comment, err := models.InsertComment(ctx, db, func(c *models.Comment) {
		c.AuthorID = inserted.ID
		c.CommentableType = "Post"
		c.CommentableID = post.ID
	})
	check("InsertComment factory", err)
	assert("InsertComment ID populated", comment.ID != "")

	// InsertPostTag.
	pt, err := models.InsertPostTag(ctx, db, func(pt *models.PostTag) {
		pt.PostID = post.ID
		pt.TagID = tag.ID
	})
	check("InsertPostTag factory", err)
	assert("InsertPostTag correct IDs", pt.PostID == post.ID && pt.TagID == tag.ID)

	// fake.* generators.
	assert("fake.String prefix", strings.HasPrefix(fake.String("test"), "test_"))
	assert("fake.Int16 > 0", fake.Int16() > 0)
	assert("fake.Int32 > 0", fake.Int32() > 0)
	assert("fake.Int64 > 0", fake.Int64() > 0)
	assert("fake.Float32 > 0", fake.Float32() > 0)
	assert("fake.Float64 > 0", fake.Float64() > 0)
	_ = fake.Bool() // just verify it doesn't panic
	assert("fake.Time not zero", !fake.Time().IsZero())
	assert("fake.UUID not empty", fake.UUID() != "")
	assert("fake.Bytes length", len(fake.Bytes(16)) == 16)
	assert("fake.JSON valid", json.Valid(fake.JSON()))
	assert("fake.Numeric not empty", fake.Numeric() != "")
	p := fake.Ptr(42)
	assert("fake.Ptr", p != nil && *p == 42)
}

// ---------------------------------------------------------------------------
// 19. Debug Executor
// ---------------------------------------------------------------------------

func testDebugExecutor(ctx context.Context, db *sql.DB) {
	section("Debug Executor")
	truncateAll(db)

	insertTestUser(ctx, db, "debug@example.com", "DebugUser")

	// DebugTo with buffer.
	var buf bytes.Buffer
	debugExec := sqlgen.DebugTo(db, &buf)

	_, err := models.AllUsers(ctx, debugExec)
	check("DebugTo AllUsers", err)
	assert("DebugTo captured SQL", strings.Contains(buf.String(), "SELECT"))
	assert("DebugTo captured SQL keyword", strings.Contains(buf.String(), "SQL:"))

	// Debug (just verify it returns a valid executor).
	debugStderr := sqlgen.Debug(db)
	_, err = models.CountUsers(ctx, debugStderr)
	check("Debug executor works", err)
}

// ---------------------------------------------------------------------------
// 20. Bind/Scan
// ---------------------------------------------------------------------------

func testBindScan(ctx context.Context, db *sql.DB) {
	section("Bind/Scan")
	truncateAll(db)

	u1 := insertTestUser(ctx, db, "bind1@example.com", "Bind1")
	insertTestUser(ctx, db, "bind2@example.com", "Bind2")

	// FieldPointers.
	ptrs, err := sqlgen.FieldPointers(u1, []string{"id", "email", "name"})
	check("FieldPointers", err)
	assert("FieldPointers length", len(ptrs) == 3)

	// ColumnMap.
	cm := sqlgen.NewColumnMap("id", "email", "name", "bio")
	assert("ColumnMap.Index(email) == 1", cm.Index("email") == 1)
	assert("ColumnMap.Index(missing) == -1", cm.Index("missing") == -1)
	assert("ColumnMap.Columns length", len(cm.Columns) == 4)

	// Bind (slice).
	q := models.Users(sqlgen.OrderBy("\"email\" ASC"))
	var users []*models.User
	check("Bind slice", sqlgen.Bind(ctx, db, q, &users))
	assert("Bind slice count", len(users) == 2)
	assert("Bind slice first email", users[0].Email == "bind1@example.com")

	// Bind (single struct).
	q = models.Users(sqlgen.Where("\"email\" = ?", "bind1@example.com"), sqlgen.Limit(1))
	var singleUser models.User
	check("Bind single", sqlgen.Bind(ctx, db, q, &singleUser))
	assert("Bind single email", singleUser.Email == "bind1@example.com")

	// ScanRows.
	q = models.Users(sqlgen.OrderBy("\"email\" ASC"))
	query, args := q.BuildSelect()
	rows, err := db.QueryContext(ctx, query, args...)
	check("ScanRows query", err)
	if err == nil {
		scanned, err := sqlgen.ScanRows(rows, func(r *sql.Rows) (*models.User, error) {
			u := &models.User{}
			err := u.ScanRow(r)
			return u, err
		})
		check("ScanRows", err)
		assert("ScanRows count", len(scanned) == 2)
	}

	// ScanOne.
	q = models.Users(sqlgen.Where("\"email\" = ?", "bind1@example.com"))
	query, args = q.BuildSelect()
	rows, err = db.QueryContext(ctx, query, args...)
	check("ScanOne query", err)
	if err == nil {
		one, err := sqlgen.ScanOne(rows, func(r *sql.Rows) (*models.User, error) {
			u := &models.User{}
			err := u.ScanRow(r)
			return u, err
		})
		check("ScanOne", err)
		assert("ScanOne email", one.Email == "bind1@example.com")
	}

	// ScanRow (single row helper).
	row := db.QueryRowContext(ctx, "SELECT 42::int")
	var val int
	err = sqlgen.ScanRow(row, &val)
	check("ScanRow", err)
	assert("ScanRow value", val == 42)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
