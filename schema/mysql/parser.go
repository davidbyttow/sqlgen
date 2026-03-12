// Package mysql implements a hand-written DDL parser for MySQL.
// It produces the same schema.Schema IR as the Postgres parser,
// so the code generator works identically for both dialects.
package mysql

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/davidbyttow/sqlgen/schema"
)

// Parser implements schema.Parser for MySQL DDL.
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

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	merged := &schema.Schema{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		s, err := p.ParseFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		merged.Tables = append(merged.Tables, s.Tables...)
		merged.Enums = append(merged.Enums, s.Enums...)
		merged.Views = append(merged.Views, s.Views...)
	}
	return merged, nil
}

func (p *Parser) ParseString(sql string) (*schema.Schema, error) {
	s := &schema.Schema{}
	tableMap := map[string]*schema.Table{}

	stmts := splitStatements(sql)
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		upper := strings.ToUpper(stmt)

		switch {
		case strings.HasPrefix(upper, "CREATE TABLE"):
			t, enums, err := parseCreateTable(stmt)
			if err != nil {
				return nil, err
			}
			s.Tables = append(s.Tables, t)
			s.Enums = append(s.Enums, enums...)
			tableMap[t.Name] = t

		case strings.HasPrefix(upper, "CREATE VIEW") || strings.HasPrefix(upper, "CREATE OR REPLACE VIEW"):
			v := parseCreateView(stmt)
			if v != nil {
				s.Views = append(s.Views, v)
			}

		case strings.HasPrefix(upper, "CREATE UNIQUE INDEX"):
			applyCreateIndex(stmt, tableMap, true)

		case strings.HasPrefix(upper, "CREATE INDEX"):
			applyCreateIndex(stmt, tableMap, false)

		case strings.HasPrefix(upper, "ALTER TABLE"):
			applyAlterTable(stmt, tableMap)
		}
	}

	return s, nil
}

// splitStatements splits SQL text on semicolons, respecting quotes and parentheses.
func splitStatements(sql string) []string {
	var stmts []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	runes := []rune(sql)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Line comment.
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}

		// Block comment.
		if inBlockComment {
			if ch == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		// Check for comment starts (only outside quotes).
		if !inSingleQuote && !inDoubleQuote && !inBacktick {
			if ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
				inLineComment = true
				i++
				continue
			}
			if ch == '#' {
				inLineComment = true
				continue
			}
			if ch == '/' && i+1 < len(runes) && runes[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		// Quote tracking.
		if !inDoubleQuote && !inBacktick && ch == '\'' {
			// Check for escaped quote.
			if inSingleQuote && i+1 < len(runes) && runes[i+1] == '\'' {
				current.WriteRune(ch)
				current.WriteRune(runes[i+1])
				i++
				continue
			}
			inSingleQuote = !inSingleQuote
			current.WriteRune(ch)
			continue
		}
		if !inSingleQuote && !inBacktick && ch == '"' {
			inDoubleQuote = !inDoubleQuote
			current.WriteRune(ch)
			continue
		}
		if !inSingleQuote && !inDoubleQuote && ch == '`' {
			inBacktick = !inBacktick
			current.WriteRune(ch)
			continue
		}

		// Semicolon outside quotes = statement boundary.
		if !inSingleQuote && !inDoubleQuote && !inBacktick && ch == ';' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				stmts = append(stmts, s)
			}
			current.Reset()
			continue
		}

		current.WriteRune(ch)
	}

	// Remaining text (no trailing semicolon).
	s := strings.TrimSpace(current.String())
	if s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// tokenize splits a SQL statement into tokens for parsing.
// Tokens are: identifiers, keywords, numbers, strings ('...'), backtick-quoted (`...`),
// and punctuation characters.
func tokenize(sql string) []string {
	var tokens []string
	runes := []rune(sql)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Skip whitespace.
		if unicode.IsSpace(ch) {
			i++
			continue
		}

		// Single-quoted string.
		if ch == '\'' {
			var buf strings.Builder
			buf.WriteRune(ch)
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					buf.WriteRune(runes[i])
					i++
					// Escaped quote ('').
					if i < len(runes) && runes[i] == '\'' {
						buf.WriteRune(runes[i])
						i++
						continue
					}
					break
				}
				buf.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, buf.String())
			continue
		}

		// Backtick-quoted identifier.
		if ch == '`' {
			var buf strings.Builder
			buf.WriteRune(ch)
			i++
			for i < len(runes) && runes[i] != '`' {
				buf.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				buf.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, buf.String())
			continue
		}

		// Double-quoted string.
		if ch == '"' {
			var buf strings.Builder
			buf.WriteRune(ch)
			i++
			for i < len(runes) && runes[i] != '"' {
				buf.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				buf.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, buf.String())
			continue
		}

		// Punctuation.
		if ch == '(' || ch == ')' || ch == ',' || ch == '=' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}

		// Word or number.
		if unicode.IsLetter(ch) || ch == '_' || unicode.IsDigit(ch) || ch == '.' {
			var buf strings.Builder
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_' || runes[i] == '.') {
				buf.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, buf.String())
			continue
		}

		// Skip anything else.
		i++
	}

	return tokens
}

// unquote strips backticks or double quotes from an identifier.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '`' && s[len(s)-1] == '`') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// unquoteString strips single quotes and unescapes doubled quotes.
func unquoteString(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		inner := s[1 : len(s)-1]
		return strings.ReplaceAll(inner, "''", "'")
	}
	return s
}

// colMeta holds column-level constraint info discovered during parsing.
type colMeta struct {
	isPK     bool
	isUnique bool
	fk       *schema.ForeignKey
}

// parseCreateTable parses a CREATE TABLE statement.
// Returns the table and any enum types extracted from ENUM(...) columns.
func parseCreateTable(stmt string) (*schema.Table, []*schema.Enum, error) {
	tokens := tokenize(stmt)
	if len(tokens) < 3 {
		return nil, nil, fmt.Errorf("invalid CREATE TABLE: too few tokens")
	}

	// Find table name: CREATE [TEMPORARY] TABLE [IF NOT EXISTS] name
	idx := 0
	for idx < len(tokens) && strings.ToUpper(tokens[idx]) != "TABLE" {
		idx++
	}
	idx++ // skip "TABLE"

	// Skip IF NOT EXISTS.
	if idx+2 < len(tokens) && strings.ToUpper(tokens[idx]) == "IF" &&
		strings.ToUpper(tokens[idx+1]) == "NOT" && strings.ToUpper(tokens[idx+2]) == "EXISTS" {
		idx += 3
	}

	if idx >= len(tokens) {
		return nil, nil, fmt.Errorf("invalid CREATE TABLE: no table name")
	}

	tableName := unquote(tokens[idx])
	idx++

	t := &schema.Table{Name: tableName}
	var enums []*schema.Enum

	// Find the opening paren of the column/constraint definitions.
	parenStart := idx
	for parenStart < len(tokens) && tokens[parenStart] != "(" {
		parenStart++
	}
	if parenStart >= len(tokens) {
		return t, nil, nil // CREATE TABLE with no columns (unlikely but safe)
	}

	// Find the matching closing paren.
	parenEnd := findMatchingParen(tokens, parenStart)
	if parenEnd < 0 {
		return nil, nil, fmt.Errorf("unmatched parenthesis in CREATE TABLE %s", tableName)
	}

	// Split the content between parens into comma-separated items (respecting nested parens).
	items := splitTopLevel(tokens[parenStart+1 : parenEnd])

	// Collect inline column constraint metadata for post-pass.
	colMetas := map[string]*colMeta{}

	for _, item := range items {
		if len(item) == 0 {
			continue
		}

		first := strings.ToUpper(item[0])

		switch {
		case first == "PRIMARY":
			pk := parseTablePrimaryKey(item)
			if pk != nil {
				t.PrimaryKey = pk
				for _, colName := range pk.Columns {
					if c := t.FindColumn(colName); c != nil {
						c.IsNullable = false
					}
				}
			}

		case first == "UNIQUE":
			uc := parseTableUnique(item)
			if uc != nil {
				t.Unique = append(t.Unique, uc)
			}

		case first == "CONSTRAINT":
			parseTableConstraint(t, item)

		case first == "KEY" || first == "INDEX":
			newIdx := parseTableIndex(item)
			if newIdx != nil {
				t.Indexes = append(t.Indexes, newIdx)
			}

		case first == "FOREIGN":
			fk := parseTableForeignKey(item)
			if fk != nil {
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}

		case first == "CHECK":
			// Parse and discard.

		default:
			// Column definition.
			col, enum, meta := parseColumnDef(item, tableName)
			if col != nil {
				t.Columns = append(t.Columns, col)
				if meta != nil {
					colMetas[col.Name] = meta
				}
			}
			if enum != nil {
				enums = append(enums, enum)
			}
		}
	}

	// Post-pass: apply inline column constraints.
	for colName, meta := range colMetas {
		if meta.isPK && t.PrimaryKey == nil {
			t.PrimaryKey = &schema.PrimaryKey{Columns: []string{colName}}
		}
		if meta.isUnique {
			t.Unique = append(t.Unique, &schema.UniqueConstraint{Columns: []string{colName}})
		}
		if meta.fk != nil {
			t.ForeignKeys = append(t.ForeignKeys, meta.fk)
		}
	}

	return t, enums, nil
}

// parseColumnDef parses a single column definition from tokens.
// Returns the column, optional enum type (for inline ENUM), and optional metadata about inline constraints.
func parseColumnDef(tokens []string, tableName string) (*schema.Column, *schema.Enum, *colMeta) {
	if len(tokens) < 2 {
		return nil, nil, nil
	}

	col := &schema.Column{
		Name:       unquote(tokens[0]),
		IsNullable: true,
	}

	idx := 1
	var enumType *schema.Enum
	meta := &colMeta{}

	// Parse the column type.
	if idx >= len(tokens) {
		return col, nil, nil
	}

	typeToken := strings.ToUpper(tokens[idx])

	// Handle ENUM(...) inline type.
	if typeToken == "ENUM" {
		idx++
		if idx < len(tokens) && tokens[idx] == "(" {
			end := findMatchingParen(tokens, idx)
			if end > idx {
				var vals []string
				for j := idx + 1; j < end; j++ {
					if tokens[j] == "," {
						continue
					}
					vals = append(vals, unquoteString(tokens[j]))
				}
				enumName := tableName + "_" + col.Name
				enumType = &schema.Enum{
					Name:   enumName,
					Values: vals,
				}
				col.DBType = "enum"
				col.EnumName = enumName
				idx = end + 1
			}
		}
	} else {
		col.DBType = normalizeMySQLType(tokens, &idx)
	}

	// TINYINT(1) is the MySQL boolean convention.
	if col.DBType == "tinyint" {
		// Check if the original type had (1) by looking at the raw tokens.
		// We already consumed type params, but we can detect it via the token right after type name.
		// Actually let's check the raw tokens for TINYINT(1) pattern.
		for j := 1; j < len(tokens)-2; j++ {
			if strings.ToUpper(tokens[j]) == "TINYINT" && j+1 < len(tokens) && tokens[j+1] == "(" {
				// Look for "1" then ")".
				if j+3 < len(tokens) && tokens[j+2] == "1" && tokens[j+3] == ")" {
					col.DBType = "boolean"
				}
				break
			}
		}
	}

	// Parse column modifiers.
	for idx < len(tokens) {
		mod := strings.ToUpper(tokens[idx])

		switch mod {
		case "NOT":
			if idx+1 < len(tokens) && strings.ToUpper(tokens[idx+1]) == "NULL" {
				col.IsNullable = false
				idx += 2
			} else {
				idx++
			}

		case "NULL":
			col.IsNullable = true
			idx++

		case "DEFAULT":
			col.HasDefault = true
			idx++
			if idx < len(tokens) {
				if tokens[idx] == "(" || (idx+1 < len(tokens) && tokens[idx+1] == "(") {
					start := idx
					if tokens[idx] != "(" {
						idx++
					}
					if idx < len(tokens) && tokens[idx] == "(" {
						end := findMatchingParen(tokens, idx)
						if end > 0 {
							idx = end + 1
						} else {
							idx++
						}
					}
					_ = start
				} else {
					col.DefaultVal = unquoteString(tokens[idx])
					idx++
				}
			}

		case "AUTO_INCREMENT":
			col.IsAutoIncrement = true
			col.HasDefault = true
			idx++

		case "PRIMARY":
			if idx+1 < len(tokens) && strings.ToUpper(tokens[idx+1]) == "KEY" {
				col.IsNullable = false
				meta.isPK = true
				idx += 2
			} else {
				idx++
			}

		case "UNIQUE":
			meta.isUnique = true
			idx++
			if idx < len(tokens) && strings.ToUpper(tokens[idx]) == "KEY" {
				idx++
			}

		case "KEY":
			idx++

		case "UNSIGNED":
			idx++

		case "ZEROFILL":
			idx++

		case "CHARACTER", "CHARSET":
			idx++
			if idx < len(tokens) && strings.ToUpper(tokens[idx]) == "SET" {
				idx++
			}
			if idx < len(tokens) {
				idx++
			}

		case "COLLATE":
			idx++
			if idx < len(tokens) {
				idx++
			}

		case "COMMENT":
			idx++
			if idx < len(tokens) {
				idx++
			}

		case "ON":
			// ON UPDATE CURRENT_TIMESTAMP, etc.
			idx++
			if idx < len(tokens) {
				idx++
			}
			if idx < len(tokens) {
				idx++
				if idx < len(tokens) && tokens[idx] == "(" {
					end := findMatchingParen(tokens, idx)
					if end > 0 {
						idx = end + 1
					}
				}
			}

		case "REFERENCES":
			fk := parseInlineFK(tokens[idx:], col.Name)
			if fk != nil {
				meta.fk = fk
			}
			idx = len(tokens)

		case "GENERATED":
			idx++
			for idx < len(tokens) {
				if tokens[idx] == "(" {
					end := findMatchingParen(tokens, idx)
					if end > 0 {
						idx = end + 1
					} else {
						idx++
					}
					break
				}
				idx++
			}
			if idx < len(tokens) {
				upper := strings.ToUpper(tokens[idx])
				if upper == "STORED" || upper == "VIRTUAL" {
					idx++
				}
			}

		case "CONSTRAINT":
			idx++
			if idx < len(tokens) {
				idx++
			}

		default:
			idx++
		}
	}

	hasMeta := meta.isPK || meta.isUnique || meta.fk != nil
	if !hasMeta {
		meta = nil
	}

	return col, enumType, meta
}

// normalizeMySQLType reads type tokens starting at idx and returns the normalized DB type.
// Advances idx past the type tokens consumed.
func normalizeMySQLType(tokens []string, idx *int) string {
	if *idx >= len(tokens) {
		return "text"
	}

	raw := strings.ToUpper(tokens[*idx])
	*idx++

	// Handle type parameters: INT(11), VARCHAR(255), DECIMAL(10,2), etc.
	// We consume but largely discard length/precision for type mapping.
	consumeTypeParams := func() {
		if *idx < len(tokens) && tokens[*idx] == "(" {
			end := findMatchingParen(tokens, *idx)
			if end > 0 {
				*idx = end + 1
			}
		}
	}

	// Check for UNSIGNED after consuming params.
	checkUnsigned := func(base string) string {
		consumeTypeParams()
		if *idx < len(tokens) && strings.ToUpper(tokens[*idx]) == "UNSIGNED" {
			*idx++
			return base + " unsigned"
		}
		return base
	}

	switch raw {
	// Integer types.
	case "TINYINT":
		consumeTypeParams()
		unsigned := false
		if *idx < len(tokens) && strings.ToUpper(tokens[*idx]) == "UNSIGNED" {
			unsigned = true
			*idx++
		}
		// TINYINT(1) is the MySQL boolean convention; we map to "boolean".
		// But we check that above in the raw token. Let's check the original.
		// Actually we need to check the param. Let's look back.
		if !unsigned {
			return "tinyint"
		}
		return "tinyint unsigned"

	case "SMALLINT":
		return checkUnsigned("smallint")

	case "MEDIUMINT":
		return checkUnsigned("mediumint")

	case "INT", "INTEGER":
		return checkUnsigned("integer")

	case "BIGINT":
		return checkUnsigned("bigint")

	case "FLOAT":
		return checkUnsigned("float")

	case "DOUBLE":
		// DOUBLE [PRECISION]
		if *idx < len(tokens) && strings.ToUpper(tokens[*idx]) == "PRECISION" {
			*idx++
		}
		return checkUnsigned("double")

	case "DECIMAL", "NUMERIC", "DEC", "FIXED":
		return checkUnsigned("decimal")

	case "BIT":
		consumeTypeParams()
		return "bit"

	// Boolean.
	case "BOOLEAN", "BOOL":
		return "boolean"

	// String types.
	case "CHAR":
		consumeTypeParams()
		return "char"

	case "VARCHAR":
		consumeTypeParams()
		return "varchar"

	case "TINYTEXT":
		return "tinytext"

	case "TEXT":
		consumeTypeParams()
		return "text"

	case "MEDIUMTEXT":
		return "mediumtext"

	case "LONGTEXT":
		return "longtext"

	// Binary types.
	case "BINARY":
		consumeTypeParams()
		return "binary"

	case "VARBINARY":
		consumeTypeParams()
		return "varbinary"

	case "TINYBLOB":
		return "tinyblob"

	case "BLOB":
		consumeTypeParams()
		return "blob"

	case "MEDIUMBLOB":
		return "mediumblob"

	case "LONGBLOB":
		return "longblob"

	// Date/time types.
	case "DATE":
		return "date"

	case "DATETIME":
		consumeTypeParams()
		return "datetime"

	case "TIMESTAMP":
		consumeTypeParams()
		return "timestamp"

	case "TIME":
		consumeTypeParams()
		return "time"

	case "YEAR":
		consumeTypeParams()
		return "year"

	// JSON.
	case "JSON":
		return "json"

	// Spatial (pass through).
	case "GEOMETRY", "POINT", "LINESTRING", "POLYGON":
		return strings.ToLower(raw)

	// SET type (similar to ENUM).
	case "SET":
		consumeTypeParams()
		return "set"

	default:
		consumeTypeParams()
		return strings.ToLower(raw)
	}
}

// parseTablePrimaryKey parses: PRIMARY KEY (col1, col2, ...)
func parseTablePrimaryKey(tokens []string) *schema.PrimaryKey {
	cols := extractParenColumns(tokens)
	if len(cols) == 0 {
		return nil
	}
	return &schema.PrimaryKey{Columns: cols}
}

// parseTableUnique parses: UNIQUE [KEY|INDEX] [name] (col1, col2, ...)
func parseTableUnique(tokens []string) *schema.UniqueConstraint {
	idx := 1 // skip UNIQUE
	var name string

	if idx < len(tokens) {
		upper := strings.ToUpper(tokens[idx])
		if upper == "KEY" || upper == "INDEX" {
			idx++
		}
	}
	// Optional constraint/index name.
	if idx < len(tokens) && tokens[idx] != "(" {
		name = unquote(tokens[idx])
		idx++
	}

	cols := extractParenColumnsFrom(tokens, idx)
	if len(cols) == 0 {
		return nil
	}
	return &schema.UniqueConstraint{Name: name, Columns: cols}
}

// parseTableIndex parses: KEY|INDEX [name] (col1, col2, ...)
func parseTableIndex(tokens []string) *schema.Index {
	idx := 1 // skip KEY/INDEX
	var name string

	// Optional index name.
	if idx < len(tokens) && tokens[idx] != "(" {
		name = unquote(tokens[idx])
		idx++
	}

	cols := extractParenColumnsFrom(tokens, idx)
	if len(cols) == 0 {
		return nil
	}
	return &schema.Index{Name: name, Columns: cols}
}

// parseTableForeignKey parses: FOREIGN KEY [name] (cols) REFERENCES table(cols) [ON ...]
func parseTableForeignKey(tokens []string) *schema.ForeignKey {
	idx := 1 // skip FOREIGN
	if idx < len(tokens) && strings.ToUpper(tokens[idx]) == "KEY" {
		idx++
	}

	fk := &schema.ForeignKey{}

	// Optional constraint name before parens.
	if idx < len(tokens) && tokens[idx] != "(" {
		// Could be a name or just parens.
		upper := strings.ToUpper(tokens[idx])
		if upper != "(" {
			fk.Name = unquote(tokens[idx])
			idx++
		}
	}

	// Local columns.
	if idx < len(tokens) && tokens[idx] == "(" {
		end := findMatchingParen(tokens, idx)
		if end > idx {
			fk.Columns = extractColumnsInRange(tokens, idx+1, end)
			idx = end + 1
		}
	}

	// REFERENCES table(cols)
	if idx < len(tokens) && strings.ToUpper(tokens[idx]) == "REFERENCES" {
		idx++
		if idx < len(tokens) {
			fk.RefTable = unquote(tokens[idx])
			idx++
		}
		if idx < len(tokens) && tokens[idx] == "(" {
			end := findMatchingParen(tokens, idx)
			if end > idx {
				fk.RefColumns = extractColumnsInRange(tokens, idx+1, end)
				idx = end + 1
			}
		}
	}

	// ON DELETE / ON UPDATE actions.
	for idx+2 < len(tokens) {
		if strings.ToUpper(tokens[idx]) != "ON" {
			break
		}
		action := strings.ToUpper(tokens[idx+1])
		idx += 2
		fkAction := parseFKAction(tokens, &idx)

		switch action {
		case "DELETE":
			fk.OnDelete = fkAction
		case "UPDATE":
			fk.OnUpdate = fkAction
		}
	}

	if len(fk.Columns) == 0 {
		return nil
	}
	return fk
}

// parseTableConstraint parses: CONSTRAINT name PRIMARY KEY|UNIQUE|FOREIGN KEY ...
func parseTableConstraint(t *schema.Table, tokens []string) {
	if len(tokens) < 3 {
		return
	}

	idx := 1 // skip CONSTRAINT
	name := unquote(tokens[idx])
	idx++

	upper := strings.ToUpper(tokens[idx])

	switch upper {
	case "PRIMARY":
		// CONSTRAINT name PRIMARY KEY (cols)
		pk := parseTablePrimaryKey(tokens[idx:])
		if pk != nil {
			pk.Name = name
			t.PrimaryKey = pk
			for _, colName := range pk.Columns {
				if c := t.FindColumn(colName); c != nil {
					c.IsNullable = false
				}
			}
		}

	case "UNIQUE":
		uc := parseTableUnique(tokens[idx:])
		if uc != nil {
			uc.Name = name
			t.Unique = append(t.Unique, uc)
		}

	case "FOREIGN":
		fk := parseTableForeignKey(tokens[idx:])
		if fk != nil {
			fk.Name = name
			t.ForeignKeys = append(t.ForeignKeys, fk)
		}

	case "CHECK":
		// Ignore CHECK constraints.
	}
}

// parseCreateView parses a CREATE VIEW statement.
func parseCreateView(stmt string) *schema.View {
	tokens := tokenize(stmt)
	idx := 0

	// Skip CREATE [OR REPLACE]
	for idx < len(tokens) && strings.ToUpper(tokens[idx]) != "VIEW" {
		idx++
	}
	idx++ // skip VIEW

	if idx >= len(tokens) {
		return nil
	}

	name := unquote(tokens[idx])
	idx++

	// Find AS keyword.
	var query string
	for idx < len(tokens) {
		if strings.ToUpper(tokens[idx]) == "AS" {
			idx++
			// Everything after AS is the query. Reconstruct from original statement.
			asPos := strings.Index(strings.ToUpper(stmt), " AS ")
			if asPos >= 0 {
				query = strings.TrimSpace(stmt[asPos+4:])
			}
			break
		}
		idx++
	}

	return &schema.View{
		Name:  name,
		Query: query,
	}
}

// applyCreateIndex parses: CREATE [UNIQUE] INDEX name ON table (cols)
func applyCreateIndex(stmt string, tableMap map[string]*schema.Table, unique bool) {
	tokens := tokenize(stmt)
	idx := 0

	// Skip to INDEX keyword.
	for idx < len(tokens) && strings.ToUpper(tokens[idx]) != "INDEX" {
		idx++
	}
	idx++ // skip INDEX

	// Optional IF NOT EXISTS.
	if idx+2 < len(tokens) && strings.ToUpper(tokens[idx]) == "IF" {
		idx += 3 // IF NOT EXISTS
	}

	if idx >= len(tokens) {
		return
	}

	indexName := unquote(tokens[idx])
	idx++

	// ON table
	if idx >= len(tokens) || strings.ToUpper(tokens[idx]) != "ON" {
		return
	}
	idx++

	if idx >= len(tokens) {
		return
	}
	tableName := unquote(tokens[idx])
	idx++

	t, ok := tableMap[tableName]
	if !ok {
		return
	}

	cols := extractParenColumnsFrom(tokens, idx)
	if len(cols) == 0 {
		return
	}

	newIdx := &schema.Index{
		Name:    indexName,
		Columns: cols,
		Unique:  unique,
	}
	t.Indexes = append(t.Indexes, newIdx)

	if unique {
		t.Unique = append(t.Unique, &schema.UniqueConstraint{
			Name:    indexName,
			Columns: cols,
		})
	}
}

// applyAlterTable handles ALTER TABLE statements (ADD CONSTRAINT, ADD INDEX, etc.)
func applyAlterTable(stmt string, tableMap map[string]*schema.Table) {
	tokens := tokenize(stmt)
	idx := 0

	// ALTER TABLE name
	for idx < len(tokens) && strings.ToUpper(tokens[idx]) != "TABLE" {
		idx++
	}
	idx++

	if idx >= len(tokens) {
		return
	}
	tableName := unquote(tokens[idx])
	idx++

	t, ok := tableMap[tableName]
	if !ok {
		return
	}

	// Process ADD clauses.
	for idx < len(tokens) {
		upper := strings.ToUpper(tokens[idx])
		if upper == "ADD" {
			idx++
			if idx >= len(tokens) {
				break
			}

			next := strings.ToUpper(tokens[idx])
			switch next {
			case "CONSTRAINT":
				parseTableConstraint(t, tokens[idx:])
				return

			case "PRIMARY":
				pk := parseTablePrimaryKey(tokens[idx:])
				if pk != nil {
					t.PrimaryKey = pk
				}
				return

			case "UNIQUE":
				uc := parseTableUnique(tokens[idx:])
				if uc != nil {
					t.Unique = append(t.Unique, uc)
				}
				return

			case "FOREIGN":
				fk := parseTableForeignKey(tokens[idx:])
				if fk != nil {
					t.ForeignKeys = append(t.ForeignKeys, fk)
				}
				return

			case "INDEX", "KEY":
				newIdx := parseTableIndex(tokens[idx:])
				if newIdx != nil {
					t.Indexes = append(t.Indexes, newIdx)
				}
				return
			}
		}
		idx++
	}
}

// parseInlineFK parses an inline REFERENCES clause for a column.
func parseInlineFK(tokens []string, colName string) *schema.ForeignKey {
	if len(tokens) == 0 || strings.ToUpper(tokens[0]) != "REFERENCES" {
		return nil
	}

	fk := &schema.ForeignKey{
		Columns: []string{colName},
	}

	idx := 1
	if idx < len(tokens) {
		fk.RefTable = unquote(tokens[idx])
		idx++
	}
	if idx < len(tokens) && tokens[idx] == "(" {
		end := findMatchingParen(tokens, idx)
		if end > idx {
			fk.RefColumns = extractColumnsInRange(tokens, idx+1, end)
			idx = end + 1
		}
	}

	for idx+2 < len(tokens) {
		if strings.ToUpper(tokens[idx]) != "ON" {
			break
		}
		action := strings.ToUpper(tokens[idx+1])
		idx += 2
		fkAction := parseFKAction(tokens, &idx)
		switch action {
		case "DELETE":
			fk.OnDelete = fkAction
		case "UPDATE":
			fk.OnUpdate = fkAction
		}
	}

	return fk
}

// parseFKAction reads a foreign key action from tokens at idx.
func parseFKAction(tokens []string, idx *int) schema.Action {
	if *idx >= len(tokens) {
		return schema.ActionNone
	}
	upper := strings.ToUpper(tokens[*idx])
	switch upper {
	case "CASCADE":
		*idx++
		return schema.ActionCascade
	case "RESTRICT":
		*idx++
		return schema.ActionRestrict
	case "SET":
		*idx++
		if *idx < len(tokens) {
			next := strings.ToUpper(tokens[*idx])
			*idx++
			switch next {
			case "NULL":
				return schema.ActionSetNull
			case "DEFAULT":
				return schema.ActionSetDefault
			}
		}
		return schema.ActionNone
	case "NO":
		*idx++
		if *idx < len(tokens) && strings.ToUpper(tokens[*idx]) == "ACTION" {
			*idx++
		}
		return schema.ActionNoAction
	default:
		*idx++
		return schema.ActionNone
	}
}

// Helper: find the matching closing paren for an opening paren at tokens[start].
func findMatchingParen(tokens []string, start int) int {
	if start >= len(tokens) || tokens[start] != "(" {
		return -1
	}
	depth := 0
	for i := start; i < len(tokens); i++ {
		if tokens[i] == "(" {
			depth++
		} else if tokens[i] == ")" {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitTopLevel splits tokens into comma-separated groups, respecting nested parens.
func splitTopLevel(tokens []string) [][]string {
	var items [][]string
	var current []string
	depth := 0

	for _, tok := range tokens {
		if tok == "(" {
			depth++
		} else if tok == ")" {
			depth--
		}

		if tok == "," && depth == 0 {
			if len(current) > 0 {
				items = append(items, current)
			}
			current = nil
			continue
		}
		current = append(current, tok)
	}
	if len(current) > 0 {
		items = append(items, current)
	}
	return items
}

// extractParenColumns extracts column names from the first (...) in tokens.
func extractParenColumns(tokens []string) []string {
	return extractParenColumnsFrom(tokens, 0)
}

// extractParenColumnsFrom extracts column names from the first (...) starting at idx.
func extractParenColumnsFrom(tokens []string, startIdx int) []string {
	for i := startIdx; i < len(tokens); i++ {
		if tokens[i] == "(" {
			end := findMatchingParen(tokens, i)
			if end > i {
				return extractColumnsInRange(tokens, i+1, end)
			}
		}
	}
	return nil
}

// extractColumnsInRange extracts unquoted identifier names from tokens[start:end], skipping commas.
func extractColumnsInRange(tokens []string, start, end int) []string {
	var cols []string
	for i := start; i < end; i++ {
		if tokens[i] == "," {
			continue
		}
		// Skip length specifiers in index columns like col(255).
		if tokens[i] == "(" {
			e := findMatchingParen(tokens, i)
			if e > 0 {
				i = e
			}
			continue
		}
		cols = append(cols, unquote(tokens[i]))
	}
	return cols
}
