package postgres

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/davidbyttow/sqlgen/schema"
)

// Parser implements schema.Parser for PostgreSQL DDL.
type Parser struct{}

var _ schema.Parser = (*Parser)(nil)

func (p *Parser) ParseFile(path string) (*schema.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return p.ParseString(string(data))
}

func (p *Parser) ParseDir(dir string) (*schema.Schema, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	// Sort for deterministic ordering.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	merged := &schema.Schema{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		s, err := p.ParseFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		mergeSchema(merged, s)
	}
	return merged, nil
}

func (p *Parser) ParseString(sql string) (*schema.Schema, error) {
	result, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parsing SQL: %w", err)
	}

	s := &schema.Schema{}

	// Track tables by name for ALTER TABLE lookups.
	tableMap := map[string]*schema.Table{}

	for _, rawStmt := range result.Stmts {
		node := rawStmt.Stmt
		switch {
		case node.GetCreateStmt() != nil:
			t := parseCreateTable(node.GetCreateStmt())
			s.Tables = append(s.Tables, t)
			tableMap[t.Name] = t
			if t.Schema != "" {
				tableMap[t.Schema+"."+t.Name] = t
			}

		case node.GetCreateEnumStmt() != nil:
			e := parseCreateEnum(node.GetCreateEnumStmt())
			s.Enums = append(s.Enums, e)

		case node.GetViewStmt() != nil:
			v := parseView(node.GetViewStmt(), false)
			s.Views = append(s.Views, v)

		case node.GetAlterTableStmt() != nil:
			applyAlterTable(node.GetAlterTableStmt(), tableMap)

		case node.GetCreateTableAsStmt() != nil:
			// CREATE MATERIALIZED VIEW is represented as CreateTableAsStmt.
			cas := node.GetCreateTableAsStmt()
			if cas.Objtype == pg_query.ObjectType_OBJECT_MATVIEW {
				v := parseMaterializedView(cas)
				s.Views = append(s.Views, v)
			}

		case node.GetIndexStmt() != nil:
			applyCreateIndex(node.GetIndexStmt(), tableMap)
		}
	}

	// Resolve enum references: mark columns that use enum types.
	enumNames := map[string]bool{}
	for _, e := range s.Enums {
		enumNames[e.Name] = true
		if e.Schema != "" {
			enumNames[e.Schema+"."+e.Name] = true
		}
	}
	for _, t := range s.Tables {
		for _, c := range t.Columns {
			if enumNames[c.DBType] {
				c.EnumName = c.DBType
			}
		}
	}

	return s, nil
}

func parseCreateTable(cs *pg_query.CreateStmt) *schema.Table {
	t := &schema.Table{
		Name:   cs.Relation.Relname,
		Schema: cs.Relation.Schemaname,
	}

	for _, elt := range cs.TableElts {
		if colDef := elt.GetColumnDef(); colDef != nil {
			col := parseColumn(colDef)
			t.Columns = append(t.Columns, col)

			// Process inline column constraints.
			for _, cn := range colDef.Constraints {
				ct := cn.GetConstraint()
				if ct == nil {
					continue
				}
				applyColumnConstraint(t, col, ct)
			}
		}

		// Table-level constraints in TableElts.
		if ct := elt.GetConstraint(); ct != nil {
			applyTableConstraint(t, ct)
		}
	}

	// Top-level constraints.
	for _, cn := range cs.Constraints {
		if ct := cn.GetConstraint(); ct != nil {
			applyTableConstraint(t, ct)
		}
	}

	return t
}

func parseColumn(cd *pg_query.ColumnDef) *schema.Column {
	col := &schema.Column{
		Name:       cd.Colname,
		IsNullable: true, // Default; NOT NULL constraint flips this.
	}

	if cd.TypeName != nil {
		var typeNames []string
		for _, n := range cd.TypeName.Names {
			if s := n.GetString_(); s != nil {
				typeNames = append(typeNames, s.Sval)
			}
		}

		typeName, isSerial := normalizeTypeName(typeNames)
		col.DBType = typeName
		col.IsAutoIncrement = isSerial
		if isSerial {
			col.IsNullable = false
			col.HasDefault = true
		}

		// Check for array types.
		if len(cd.TypeName.ArrayBounds) > 0 {
			col.IsArray = true
			col.ArrayDims = len(cd.TypeName.ArrayBounds)
		}
	}

	return col
}

func applyColumnConstraint(t *schema.Table, col *schema.Column, ct *pg_query.Constraint) {
	switch ct.Contype {
	case pg_query.ConstrType_CONSTR_NOTNULL:
		col.IsNullable = false

	case pg_query.ConstrType_CONSTR_PRIMARY:
		col.IsNullable = false
		t.PrimaryKey = &schema.PrimaryKey{
			Name:    ct.Conname,
			Columns: []string{col.Name},
		}

	case pg_query.ConstrType_CONSTR_UNIQUE:
		t.Unique = append(t.Unique, &schema.UniqueConstraint{
			Name:    ct.Conname,
			Columns: []string{col.Name},
		})

	case pg_query.ConstrType_CONSTR_DEFAULT:
		col.HasDefault = true
		if ct.RawExpr != nil {
			col.DefaultVal = deparseExpr(ct.RawExpr)
		}

	case pg_query.ConstrType_CONSTR_FOREIGN:
		fk := &schema.ForeignKey{
			Name:       ct.Conname,
			Columns:    []string{col.Name},
			RefTable:   ct.Pktable.Relname,
			RefSchema:  ct.Pktable.Schemaname,
			RefColumns: nodeListToStrings(ct.PkAttrs),
			OnDelete:   mapFKAction(ct.FkDelAction),
			OnUpdate:   mapFKAction(ct.FkUpdAction),
		}
		// If no ref columns specified, assume PK of referenced table.
		if len(fk.RefColumns) == 0 {
			fk.RefColumns = []string{ct.Pktable.Relname[:0]} // will be resolved later
		}
		t.ForeignKeys = append(t.ForeignKeys, fk)

	case pg_query.ConstrType_CONSTR_IDENTITY:
		col.IsAutoIncrement = true
		col.HasDefault = true
	}
}

func applyTableConstraint(t *schema.Table, ct *pg_query.Constraint) {
	switch ct.Contype {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		t.PrimaryKey = &schema.PrimaryKey{
			Name:    ct.Conname,
			Columns: nodeListToStrings(ct.Keys),
		}
		// PK columns are implicitly NOT NULL.
		for _, colName := range t.PrimaryKey.Columns {
			if c := t.FindColumn(colName); c != nil {
				c.IsNullable = false
			}
		}

	case pg_query.ConstrType_CONSTR_UNIQUE:
		t.Unique = append(t.Unique, &schema.UniqueConstraint{
			Name:    ct.Conname,
			Columns: nodeListToStrings(ct.Keys),
		})

	case pg_query.ConstrType_CONSTR_FOREIGN:
		fk := &schema.ForeignKey{
			Name:       ct.Conname,
			Columns:    nodeListToStrings(ct.FkAttrs),
			RefTable:   ct.Pktable.Relname,
			RefSchema:  ct.Pktable.Schemaname,
			RefColumns: nodeListToStrings(ct.PkAttrs),
			OnDelete:   mapFKAction(ct.FkDelAction),
			OnUpdate:   mapFKAction(ct.FkUpdAction),
		}
		t.ForeignKeys = append(t.ForeignKeys, fk)
	}
}

func applyAlterTable(as *pg_query.AlterTableStmt, tableMap map[string]*schema.Table) {
	tableName := as.Relation.Relname
	schemaName := as.Relation.Schemaname

	key := tableName
	if schemaName != "" {
		key = schemaName + "." + tableName
	}

	t, ok := tableMap[key]
	if !ok {
		// Table not found; might be defined in another file. Skip silently.
		return
	}

	for _, cmdNode := range as.Cmds {
		cmd := cmdNode.GetAlterTableCmd()
		if cmd == nil {
			continue
		}
		if ct := cmd.Def.GetConstraint(); ct != nil {
			applyTableConstraint(t, ct)
		}
	}
}

func applyCreateIndex(is *pg_query.IndexStmt, tableMap map[string]*schema.Table) {
	tableName := is.Relation.Relname
	schemaName := is.Relation.Schemaname

	key := tableName
	if schemaName != "" {
		key = schemaName + "." + tableName
	}

	t, ok := tableMap[key]
	if !ok {
		return
	}

	var cols []string
	for _, elem := range is.IndexParams {
		if ie := elem.GetIndexElem(); ie != nil && ie.Name != "" {
			cols = append(cols, ie.Name)
		}
	}

	idx := &schema.Index{
		Name:    is.Idxname,
		Columns: cols,
		Unique:  is.Unique,
	}
	t.Indexes = append(t.Indexes, idx)

	// A unique index also implies a unique constraint for relationship inference.
	if is.Unique {
		t.Unique = append(t.Unique, &schema.UniqueConstraint{
			Name:    is.Idxname,
			Columns: cols,
		})
	}
}

func parseCreateEnum(es *pg_query.CreateEnumStmt) *schema.Enum {
	var schemaName, typeName string
	names := es.TypeName
	switch len(names) {
	case 1:
		typeName = names[0].GetString_().Sval
	case 2:
		schemaName = names[0].GetString_().Sval
		typeName = names[1].GetString_().Sval
	}

	var values []string
	for _, v := range es.Vals {
		if s := v.GetString_(); s != nil {
			values = append(values, s.Sval)
		}
	}

	return &schema.Enum{
		Schema: schemaName,
		Name:   typeName,
		Values: values,
	}
}

func parseView(vs *pg_query.ViewStmt, materialized bool) *schema.View {
	v := &schema.View{
		Name:           vs.View.Relname,
		Schema:         vs.View.Schemaname,
		IsMaterialized: materialized,
	}
	if vs.Query != nil {
		v.Query = deparseQuery(vs.Query)
	}
	return v
}

func parseMaterializedView(cas *pg_query.CreateTableAsStmt) *schema.View {
	v := &schema.View{
		IsMaterialized: true,
	}
	if cas.Into != nil && cas.Into.Rel != nil {
		v.Name = cas.Into.Rel.Relname
		v.Schema = cas.Into.Rel.Schemaname
	}
	if cas.Query != nil {
		v.Query = deparseQuery(cas.Query)
	}
	return v
}

// nodeListToStrings extracts string values from a list of String nodes.
func nodeListToStrings(nodes []*pg_query.Node) []string {
	var result []string
	for _, n := range nodes {
		if s := n.GetString_(); s != nil {
			result = append(result, s.Sval)
		}
	}
	return result
}

// mapFKAction converts a pg_query FK action string to our Action type.
func mapFKAction(action string) schema.Action {
	switch strings.ToLower(action) {
	case "c":
		return schema.ActionCascade
	case "n":
		return schema.ActionSetNull
	case "d":
		return schema.ActionSetDefault
	case "r":
		return schema.ActionRestrict
	case "a":
		return schema.ActionNoAction
	default:
		return schema.ActionNone
	}
}

// deparseExpr attempts to convert an expression node back to SQL text.
// pg_query v6 only supports deparsing full statements, so for individual
// expressions we wrap them in a SELECT and extract the result.
func deparseExpr(node *pg_query.Node) string {
	// Build a minimal "SELECT <expr>" parse tree and deparse it.
	selectStmt := &pg_query.SelectStmt{
		TargetList: []*pg_query.Node{
			{Node: &pg_query.Node_ResTarget{ResTarget: &pg_query.ResTarget{Val: node}}},
		},
	}
	parseResult := &pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{
			{Stmt: &pg_query.Node{Node: &pg_query.Node_SelectStmt{SelectStmt: selectStmt}}},
		},
	}
	result, err := pg_query.Deparse(parseResult)
	if err != nil {
		return ""
	}
	// Strip "SELECT " prefix.
	result = strings.TrimPrefix(result, "SELECT ")
	return result
}

// deparseQuery deparses a full query node (e.g., a view's SELECT).
func deparseQuery(node *pg_query.Node) string {
	parseResult := &pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{
			{Stmt: node},
		},
	}
	result, err := pg_query.Deparse(parseResult)
	if err != nil {
		return ""
	}
	return result
}

// mergeSchema merges src into dst.
func mergeSchema(dst, src *schema.Schema) {
	dst.Tables = append(dst.Tables, src.Tables...)
	dst.Enums = append(dst.Enums, src.Enums...)
	dst.Views = append(dst.Views, src.Views...)
}
