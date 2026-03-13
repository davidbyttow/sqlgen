// This example demonstrates generating code from a live Postgres database
// using DSN-based introspection.
//
// Prerequisites:
//  1. A running Postgres instance
//  2. Create a database and apply the schema:
//     createdb sqlgen_example
//     psql sqlgen_example < schema.sql
//  3. Set DATABASE_URL:
//     export DATABASE_URL="postgres://localhost:5432/sqlgen_example?sslmode=disable"
//
// Then generate and run:
//
//	go run ../../cmd/sqlgen generate
//	go run .
package main

import (
	"fmt"

	"github.com/davidbyttow/sqlgen/examples/introspect/models"
	"github.com/davidbyttow/sqlgen"
)

func main() {
	// Models generated from live DB introspection are identical
	// to those generated from DDL files.

	user := models.User{
		Email: "alice@example.com",
		Name:  "Alice",
		Bio:   sqlgen.NewNull("Writes Go."),
	}
	fmt.Printf("User: %s <%s>\n", user.Name, user.Email)

	// Type-safe queries work the same way.
	q := models.Users(
		models.UserWhere.Email.EQ("alice@example.com"),
		sqlgen.Limit(1),
	)
	sql, args := q.BuildSelect()
	fmt.Printf("SQL:  %s\nArgs: %v\n", sql, args)

	// Enum types are detected from the database.
	status := models.PostStatusDraft
	fmt.Printf("Status: %s (valid: %v)\n", status, status.IsValid())
}
