package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/davidbyttow/sqlgen/schema"
)

// Introspect connects to a running Postgres database and builds a Schema IR
// by querying information_schema and pg_catalog. The DSN should be a standard
// Postgres connection string (e.g., "postgres://user:pass@localhost:5432/dbname").
func Introspect(ctx context.Context, dsn string) (*schema.Schema, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return IntrospectDB(ctx, db)
}

// IntrospectDB builds a Schema IR from an existing database connection.
// Useful when you already have a *sql.DB handle.
func IntrospectDB(ctx context.Context, db *sql.DB) (*schema.Schema, error) {
	s := &schema.Schema{}

	enums, err := introspectEnums(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspecting enums: %w", err)
	}
	s.Enums = enums

	// Build enum name set for column type detection.
	enumNames := make(map[string]bool, len(enums))
	for _, e := range enums {
		enumNames[e.Name] = true
	}

	tables, err := introspectTables(ctx, db, enumNames)
	if err != nil {
		return nil, fmt.Errorf("introspecting tables: %w", err)
	}
	s.Tables = tables

	views, err := introspectViews(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspecting views: %w", err)
	}
	s.Views = views

	return s, nil
}

func introspectEnums(ctx context.Context, db *sql.DB) ([]*schema.Enum, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT n.nspname, t.typname, e.enumlabel
		FROM pg_type t
		JOIN pg_enum e ON t.oid = e.enumtypid
		JOIN pg_namespace n ON t.typnamespace = n.oid
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY t.typname, e.enumsortorder
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type enumKey struct{ schema, name string }
	ordered := []enumKey{}
	enumMap := map[enumKey]*schema.Enum{}

	for rows.Next() {
		var ns, name, label string
		if err := rows.Scan(&ns, &name, &label); err != nil {
			return nil, err
		}
		key := enumKey{ns, name}
		e, ok := enumMap[key]
		if !ok {
			e = &schema.Enum{Schema: ns, Name: name}
			enumMap[key] = e
			ordered = append(ordered, key)
		}
		e.Values = append(e.Values, label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]*schema.Enum, 0, len(ordered))
	for _, key := range ordered {
		result = append(result, enumMap[key])
	}
	return result, nil
}

func introspectTables(ctx context.Context, db *sql.DB, enumNames map[string]bool) ([]*schema.Table, error) {
	// 1. Get tables.
	tableRows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		  AND table_type = 'BASE TABLE'
		ORDER BY table_schema, table_name
	`)
	if err != nil {
		return nil, err
	}
	defer tableRows.Close()

	type tableKey struct{ schema, name string }
	var tableOrder []tableKey
	tableMap := map[tableKey]*schema.Table{}

	for tableRows.Next() {
		var ts, tn string
		if err := tableRows.Scan(&ts, &tn); err != nil {
			return nil, err
		}
		key := tableKey{ts, tn}
		t := &schema.Table{Schema: ts, Name: tn}
		tableMap[key] = t
		tableOrder = append(tableOrder, key)
	}
	if err := tableRows.Err(); err != nil {
		return nil, err
	}

	// 2. Get columns.
	colRows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name, column_name, udt_name, data_type,
		       is_nullable, column_default, is_identity, identity_generation
		FROM information_schema.columns
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name, ordinal_position
	`)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	for colRows.Next() {
		var ts, tn, colName, udtName, dataType, isNullable string
		var colDefault, isIdentity, identityGen sql.NullString
		if err := colRows.Scan(&ts, &tn, &colName, &udtName, &dataType, &isNullable, &colDefault, &isIdentity, &identityGen); err != nil {
			return nil, err
		}

		key := tableKey{ts, tn}
		t, ok := tableMap[key]
		if !ok {
			continue // column belongs to a view or excluded table
		}

		col := &schema.Column{
			Name:       colName,
			IsNullable: isNullable == "YES",
			HasDefault: colDefault.Valid,
			DefaultVal: colDefault.String,
		}

		// Detect auto-increment: nextval() default or identity column.
		if colDefault.Valid && strings.HasPrefix(colDefault.String, "nextval(") {
			col.IsAutoIncrement = true
		}
		if identityGen.Valid && identityGen.String != "" {
			col.IsAutoIncrement = true
		}

		// Detect arrays.
		if dataType == "ARRAY" {
			col.IsArray = true
			col.ArrayDims = 1
			// udt_name for arrays has a leading underscore (e.g., _int4).
			elemType := strings.TrimPrefix(udtName, "_")
			mapped, ok := pgTypeMap[elemType]
			if ok {
				col.DBType = mapped
			} else {
				col.DBType = elemType
			}
		} else {
			// Normalize through the same type map the DDL parser uses.
			mapped, ok := pgTypeMap[udtName]
			if ok {
				col.DBType = mapped
			} else {
				col.DBType = udtName
			}
		}

		// Detect enums.
		if enumNames[udtName] {
			col.EnumName = udtName
		}

		t.Columns = append(t.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	// 3. Primary keys.
	pkRows, err := db.QueryContext(ctx, `
		SELECT tc.table_schema, tc.table_name, tc.constraint_name,
		       kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY tc.table_schema, tc.table_name, kcu.ordinal_position
	`)
	if err != nil {
		return nil, err
	}
	defer pkRows.Close()

	for pkRows.Next() {
		var ts, tn, cname, col string
		if err := pkRows.Scan(&ts, &tn, &cname, &col); err != nil {
			return nil, err
		}
		key := tableKey{ts, tn}
		t, ok := tableMap[key]
		if !ok {
			continue
		}
		if t.PrimaryKey == nil {
			t.PrimaryKey = &schema.PrimaryKey{Name: cname}
		}
		t.PrimaryKey.Columns = append(t.PrimaryKey.Columns, col)
	}
	if err := pkRows.Err(); err != nil {
		return nil, err
	}

	// 4. Foreign keys (using pg_constraint for correct composite FK ordering).
	fkRows, err := db.QueryContext(ctx, `
		SELECT
			n.nspname AS table_schema,
			cl.relname AS table_name,
			co.conname AS constraint_name,
			a1.attname AS column_name,
			nf.nspname AS ref_schema,
			clf.relname AS ref_table,
			a2.attname AS ref_column,
			co.confdeltype, co.confupdtype
		FROM pg_constraint co
		JOIN pg_class cl ON co.conrelid = cl.oid
		JOIN pg_namespace n ON cl.relnamespace = n.oid
		JOIN pg_class clf ON co.confrelid = clf.oid
		JOIN pg_namespace nf ON clf.relnamespace = nf.oid,
		LATERAL unnest(co.conkey) WITH ORDINALITY AS u1(attnum, ord),
		LATERAL unnest(co.confkey) WITH ORDINALITY AS u2(attnum, ord)
		JOIN pg_attribute a1 ON a1.attrelid = cl.oid AND a1.attnum = u1.attnum
		JOIN pg_attribute a2 ON a2.attrelid = clf.oid AND a2.attnum = u2.attnum
		WHERE co.contype = 'f'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND u1.ord = u2.ord
		ORDER BY n.nspname, cl.relname, co.conname, u1.ord
	`)
	if err != nil {
		return nil, err
	}
	defer fkRows.Close()

	type fkKey struct{ schema, table, constraint string }
	fkMap := map[fkKey]*schema.ForeignKey{}

	for fkRows.Next() {
		var ts, tn, cname, col, refSchema, refTable, refCol string
		var delType, updType string
		if err := fkRows.Scan(&ts, &tn, &cname, &col, &refSchema, &refTable, &refCol, &delType, &updType); err != nil {
			return nil, err
		}

		key := tableKey{ts, tn}
		t, ok := tableMap[key]
		if !ok {
			continue
		}

		fk := fkKey{ts, tn, cname}
		existing, ok := fkMap[fk]
		if !ok {
			existing = &schema.ForeignKey{
				Name:      cname,
				RefTable:  refTable,
				RefSchema: refSchema,
				OnDelete:  mapPgAction(delType),
				OnUpdate:  mapPgAction(updType),
			}
			fkMap[fk] = existing
			t.ForeignKeys = append(t.ForeignKeys, existing)
		}
		existing.Columns = append(existing.Columns, col)
		existing.RefColumns = append(existing.RefColumns, refCol)
	}
	if err := fkRows.Err(); err != nil {
		return nil, err
	}

	// 5. Indexes and unique constraints.
	idxRows, err := db.QueryContext(ctx, `
		SELECT
			n.nspname AS schemaname,
			ct.relname AS tablename,
			ci.relname AS indexname,
			ix.indisunique,
			a.attname
		FROM pg_index ix
		JOIN pg_class ci ON ci.oid = ix.indexrelid
		JOIN pg_class ct ON ct.oid = ix.indrelid
		JOIN pg_namespace n ON ct.relnamespace = n.oid,
		LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord)
		JOIN pg_attribute a ON a.attrelid = ct.oid AND a.attnum = k.attnum
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND NOT ix.indisprimary
		  AND k.attnum > 0
		ORDER BY n.nspname, ct.relname, ci.relname, k.ord
	`)
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	type idxKey struct{ schema, table, index string }
	type idxInfo struct {
		unique  bool
		columns []string
	}
	idxMap := map[idxKey]*idxInfo{}
	var idxOrder []idxKey

	for idxRows.Next() {
		var ns, tn, iname string
		var isUnique bool
		var col string
		if err := idxRows.Scan(&ns, &tn, &iname, &isUnique, &col); err != nil {
			return nil, err
		}
		key := idxKey{ns, tn, iname}
		info, ok := idxMap[key]
		if !ok {
			info = &idxInfo{unique: isUnique}
			idxMap[key] = info
			idxOrder = append(idxOrder, key)
		}
		info.columns = append(info.columns, col)
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	for _, key := range idxOrder {
		info := idxMap[key]
		tkey := tableKey{key.schema, key.table}
		t, ok := tableMap[tkey]
		if !ok {
			continue
		}
		idx := &schema.Index{
			Name:    key.index,
			Columns: info.columns,
			Unique:  info.unique,
		}
		t.Indexes = append(t.Indexes, idx)
		if info.unique {
			t.Unique = append(t.Unique, &schema.UniqueConstraint{
				Name:    key.index,
				Columns: info.columns,
			})
		}
	}

	// Build result in table order.
	result := make([]*schema.Table, 0, len(tableOrder))
	for _, key := range tableOrder {
		result = append(result, tableMap[key])
	}
	return result, nil
}

func introspectViews(ctx context.Context, db *sql.DB) ([]*schema.View, error) {
	// Regular views.
	viewRows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name, view_definition
		FROM information_schema.views
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name
	`)
	if err != nil {
		return nil, err
	}
	defer viewRows.Close()

	type viewKey struct{ schema, name string }
	var viewOrder []viewKey
	viewMap := map[viewKey]*schema.View{}

	for viewRows.Next() {
		var vs, vn string
		var query sql.NullString
		if err := viewRows.Scan(&vs, &vn, &query); err != nil {
			return nil, err
		}
		key := viewKey{vs, vn}
		v := &schema.View{
			Schema: vs,
			Name:   vn,
			Query:  query.String,
		}
		viewMap[key] = v
		viewOrder = append(viewOrder, key)
	}
	if err := viewRows.Err(); err != nil {
		return nil, err
	}

	// Materialized views.
	matRows, err := db.QueryContext(ctx, `
		SELECT schemaname, matviewname, definition
		FROM pg_matviews
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname, matviewname
	`)
	if err != nil {
		return nil, err
	}
	defer matRows.Close()

	for matRows.Next() {
		var vs, vn string
		var query sql.NullString
		if err := matRows.Scan(&vs, &vn, &query); err != nil {
			return nil, err
		}
		key := viewKey{vs, vn}
		v := &schema.View{
			Schema:         vs,
			Name:           vn,
			Query:          query.String,
			IsMaterialized: true,
		}
		viewMap[key] = v
		viewOrder = append(viewOrder, key)
	}
	if err := matRows.Err(); err != nil {
		return nil, err
	}

	// Get columns for all views.
	// Regular view columns from information_schema.
	colRows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name, column_name, udt_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE (table_schema, table_name) IN (
			SELECT table_schema, table_name FROM information_schema.views
			WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		)
		ORDER BY table_schema, table_name, ordinal_position
	`)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	for colRows.Next() {
		var ts, tn, colName, udtName, dataType, isNullable string
		if err := colRows.Scan(&ts, &tn, &colName, &udtName, &dataType, &isNullable); err != nil {
			return nil, err
		}
		key := viewKey{ts, tn}
		v, ok := viewMap[key]
		if !ok {
			continue
		}
		col := &schema.Column{
			Name:       colName,
			IsNullable: isNullable == "YES",
		}
		if dataType == "ARRAY" {
			col.IsArray = true
			col.ArrayDims = 1
			elemType := strings.TrimPrefix(udtName, "_")
			if mapped, ok := pgTypeMap[elemType]; ok {
				col.DBType = mapped
			} else {
				col.DBType = elemType
			}
		} else {
			if mapped, ok := pgTypeMap[udtName]; ok {
				col.DBType = mapped
			} else {
				col.DBType = udtName
			}
		}
		v.Columns = append(v.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	// Materialized view columns from pg_attribute.
	matColRows, err := db.QueryContext(ctx, `
		SELECT n.nspname, c.relname, a.attname,
		       t.typname, a.attnotnull, a.attnum
		FROM pg_attribute a
		JOIN pg_class c ON a.attrelid = c.oid
		JOIN pg_namespace n ON c.relnamespace = n.oid
		JOIN pg_type t ON a.atttypid = t.oid
		WHERE c.relkind = 'm'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY n.nspname, c.relname, a.attnum
	`)
	if err != nil {
		return nil, err
	}
	defer matColRows.Close()

	for matColRows.Next() {
		var ns, tn, colName, typName string
		var notNull bool
		var attnum int
		_ = attnum
		if err := matColRows.Scan(&ns, &tn, &colName, &typName, &notNull, &attnum); err != nil {
			return nil, err
		}
		key := viewKey{ns, tn}
		v, ok := viewMap[key]
		if !ok {
			continue
		}
		col := &schema.Column{
			Name:       colName,
			IsNullable: !notNull,
		}
		if mapped, ok := pgTypeMap[typName]; ok {
			col.DBType = mapped
		} else {
			col.DBType = typName
		}
		v.Columns = append(v.Columns, col)
	}
	if err := matColRows.Err(); err != nil {
		return nil, err
	}

	result := make([]*schema.View, 0, len(viewOrder))
	for _, key := range viewOrder {
		result = append(result, viewMap[key])
	}
	return result, nil
}

// mapPgAction converts pg_constraint action chars to schema.Action values.
func mapPgAction(code string) schema.Action {
	switch code {
	case "a":
		return schema.ActionNoAction
	case "r":
		return schema.ActionRestrict
	case "c":
		return schema.ActionCascade
	case "n":
		return schema.ActionSetNull
	case "d":
		return schema.ActionSetDefault
	default:
		return schema.ActionNone
	}
}
