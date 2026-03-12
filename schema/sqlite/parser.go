// Package sqlite implements a hand-written DDL parser for SQLite.
// SQLite DDL is simple enough that a full parser generator isn't needed.
package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/davidbyttow/sqlgen/schema"
)

// Parser implements schema.Parser for SQLite DDL.
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
			t, err := parseCreateTable(stmt)
			if err != nil {
				return nil, fmt.Errorf("parsing CREATE TABLE: %w", err)
			}
			s.Tables = append(s.Tables, t)
			tableMap[t.Name] = t

		case strings.HasPrefix(upper, "CREATE UNIQUE INDEX"):
			idx, tableName, err := parseCreateIndex(stmt, true)
			if err != nil {
				return nil, fmt.Errorf("parsing CREATE UNIQUE INDEX: %w", err)
			}
			if t, ok := tableMap[tableName]; ok {
				t.Indexes = append(t.Indexes, idx)
				t.Unique = append(t.Unique, &schema.UniqueConstraint{
					Name:    idx.Name,
					Columns: idx.Columns,
				})
			}

		case strings.HasPrefix(upper, "CREATE INDEX"):
			idx, tableName, err := parseCreateIndex(stmt, false)
			if err != nil {
				return nil, fmt.Errorf("parsing CREATE INDEX: %w", err)
			}
			if t, ok := tableMap[tableName]; ok {
				t.Indexes = append(t.Indexes, idx)
			}

		case strings.HasPrefix(upper, "CREATE VIEW"):
			v, err := parseCreateView(stmt)
			if err != nil {
				return nil, fmt.Errorf("parsing CREATE VIEW: %w", err)
			}
			s.Views = append(s.Views, v)
		}
	}

	return s, nil
}

// splitStatements splits SQL text on semicolons, respecting quoted strings
// and parenthesized groups.
func splitStatements(sql string) []string {
	var stmts []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(sql); i++ {
		ch := sql[i]

		if inSingle {
			current.WriteByte(ch)
			if ch == '\'' {
				// Check for escaped quote ('')
				if i+1 < len(sql) && sql[i+1] == '\'' {
					current.WriteByte(sql[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			current.WriteByte(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
			current.WriteByte(ch)
		case '"':
			inDouble = true
			current.WriteByte(ch)
		case '-':
			// Line comment: -- ...
			if i+1 < len(sql) && sql[i+1] == '-' {
				for i < len(sql) && sql[i] != '\n' {
					i++
				}
			} else {
				current.WriteByte(ch)
			}
		case '/':
			// Block comment: /* ... */
			if i+1 < len(sql) && sql[i+1] == '*' {
				i += 2
				for i < len(sql)-1 {
					if sql[i] == '*' && sql[i+1] == '/' {
						i++
						break
					}
					i++
				}
			} else {
				current.WriteByte(ch)
			}
		case ';':
			s := strings.TrimSpace(current.String())
			if s != "" {
				stmts = append(stmts, s)
			}
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}

	// Trailing statement without semicolon.
	s := strings.TrimSpace(current.String())
	if s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// parseCreateTable parses a CREATE TABLE statement.
func parseCreateTable(stmt string) (*schema.Table, error) {
	tok := newTokenizer(stmt)

	// Consume "CREATE TABLE"
	tok.expectKeywords("CREATE", "TABLE")

	// Optional "IF NOT EXISTS"
	tok.tryKeywords("IF", "NOT", "EXISTS")

	name := tok.nextIdent()
	t := &schema.Table{Name: name}

	if !tok.expect('(') {
		return nil, fmt.Errorf("expected '(' after table name %q", name)
	}

	// Parse column definitions and table constraints.
	elements := splitTopLevel(tok.balancedParens())

	for _, elem := range elements {
		elem = strings.TrimSpace(elem)
		if elem == "" {
			continue
		}
		upper := strings.ToUpper(elem)

		switch {
		case strings.HasPrefix(upper, "PRIMARY KEY"):
			cols := extractParenList(elem)
			t.PrimaryKey = &schema.PrimaryKey{Columns: cols}
			for _, colName := range cols {
				if c := t.FindColumn(colName); c != nil {
					c.IsNullable = false
				}
			}

		case strings.HasPrefix(upper, "FOREIGN KEY"):
			fk := parseTableForeignKey(elem)
			t.ForeignKeys = append(t.ForeignKeys, fk)

		case strings.HasPrefix(upper, "UNIQUE"):
			cols := extractParenList(elem)
			t.Unique = append(t.Unique, &schema.UniqueConstraint{Columns: cols})

		case strings.HasPrefix(upper, "CONSTRAINT"):
			parseNamedConstraint(t, elem)

		case strings.HasPrefix(upper, "CHECK"):
			// Skip CHECK constraints.

		default:
			col, colConstraints := parseColumnDef(elem)
			t.Columns = append(t.Columns, col)
			applyColumnConstraints(t, col, colConstraints)
		}
	}

	// Parse trailing options (e.g., WITHOUT ROWID).
	// We don't need these for codegen, just consume them.

	return t, nil
}

// parseColumnDef parses a single column definition like "id INTEGER PRIMARY KEY".
func parseColumnDef(def string) (*schema.Column, []string) {
	tok := newTokenizer(def)

	name := tok.nextIdent()
	col := &schema.Column{
		Name:       name,
		IsNullable: true,
	}

	// Type name. SQLite types are flexible; we grab tokens until we hit a constraint keyword
	// or end of definition.
	var typeParts []string
	for {
		saved := tok.pos
		word := tok.peek()
		if word == "" {
			break
		}
		upper := strings.ToUpper(word)

		// These keywords signal the start of constraints, not type names.
		if isConstraintKeyword(upper) {
			tok.pos = saved
			break
		}

		tok.next()

		// Handle type with parenthesized precision, e.g., VARCHAR(255) or NUMERIC(10,2)
		if tok.peekByte() == '(' {
			tok.expect('(')
			tok.balancedParens() // consume and discard precision
			// Don't append the precision to typeParts; we just need the base type name.
		}

		typeParts = append(typeParts, word)
	}

	rawType := strings.Join(typeParts, " ")
	col.DBType = normalizeType(rawType)

	// Collect constraint keywords.
	var constraints []string
	for {
		word := tok.peek()
		if word == "" {
			break
		}
		constraints = append(constraints, tok.remaining())
		break
	}

	return col, constraints
}

// applyColumnConstraints processes inline constraints on a column.
func applyColumnConstraints(t *schema.Table, col *schema.Column, constraints []string) {
	if len(constraints) == 0 {
		return
	}

	text := constraints[0]
	tok := newTokenizer(text)

	for {
		word := tok.next()
		if word == "" {
			break
		}
		upper := strings.ToUpper(word)

		switch upper {
		case "PRIMARY":
			tok.tryKeyword("KEY")
			col.IsNullable = false
			t.PrimaryKey = &schema.PrimaryKey{Columns: []string{col.Name}}
			// INTEGER PRIMARY KEY is an alias for ROWID (auto-increment).
			if strings.ToUpper(col.DBType) == "INTEGER" {
				col.IsAutoIncrement = true
				col.HasDefault = true
			}

		case "AUTOINCREMENT":
			col.IsAutoIncrement = true
			col.HasDefault = true

		case "NOT":
			tok.tryKeyword("NULL")
			col.IsNullable = false

		case "NULL":
			col.IsNullable = true

		case "DEFAULT":
			col.HasDefault = true
			col.DefaultVal = parseDefaultValue(tok)

		case "UNIQUE":
			t.Unique = append(t.Unique, &schema.UniqueConstraint{
				Columns: []string{col.Name},
			})

		case "REFERENCES":
			fk := parseInlineReference(tok, col.Name)
			t.ForeignKeys = append(t.ForeignKeys, fk)

		case "CHECK":
			// Skip the CHECK expression.
			if tok.peekByte() == '(' {
				tok.expect('(')
				tok.balancedParens()
			}

		case "CONSTRAINT":
			// Named inline constraint; consume the name and continue.
			tok.next()

		case "COLLATE":
			tok.next() // consume collation name

		case "GENERATED":
			// GENERATED ALWAYS AS (...) STORED/VIRTUAL - skip
			for {
				w := tok.next()
				if w == "" {
					break
				}
				if strings.ToUpper(w) == "STORED" || strings.ToUpper(w) == "VIRTUAL" {
					break
				}
				if tok.peekByte() == '(' {
					tok.expect('(')
					tok.balancedParens()
				}
			}
		}
	}
}

// parseInlineReference parses "REFERENCES table(col) [ON DELETE action] [ON UPDATE action]".
func parseInlineReference(tok *tokenizer, localCol string) *schema.ForeignKey {
	fk := &schema.ForeignKey{
		Columns: []string{localCol},
	}

	fk.RefTable = tok.nextIdent()

	// Optional (column_list)
	if tok.peekByte() == '(' {
		tok.expect('(')
		content := tok.balancedParens()
		fk.RefColumns = splitIdentList(content)
	}

	// ON DELETE / ON UPDATE
	parseReferentialActions(tok, fk)

	return fk
}

// parseTableForeignKey parses a table-level FOREIGN KEY constraint.
func parseTableForeignKey(def string) *schema.ForeignKey {
	tok := newTokenizer(def)

	// FOREIGN KEY (cols) REFERENCES table (cols) ...
	tok.expectKeywords("FOREIGN", "KEY")

	fk := &schema.ForeignKey{}

	if tok.expect('(') {
		fk.Columns = splitIdentList(tok.balancedParens())
	}

	tok.tryKeyword("REFERENCES")
	fk.RefTable = tok.nextIdent()

	if tok.peekByte() == '(' {
		tok.expect('(')
		fk.RefColumns = splitIdentList(tok.balancedParens())
	}

	parseReferentialActions(tok, fk)

	return fk
}

// parseNamedConstraint handles "CONSTRAINT name ..." at the table level.
func parseNamedConstraint(t *schema.Table, def string) {
	tok := newTokenizer(def)
	tok.tryKeyword("CONSTRAINT")
	constraintName := tok.nextIdent()

	keyword := strings.ToUpper(tok.next())
	switch keyword {
	case "PRIMARY":
		tok.tryKeyword("KEY")
		if tok.expect('(') {
			cols := splitIdentList(tok.balancedParens())
			t.PrimaryKey = &schema.PrimaryKey{
				Name:    constraintName,
				Columns: cols,
			}
			for _, colName := range cols {
				if c := t.FindColumn(colName); c != nil {
					c.IsNullable = false
				}
			}
		}

	case "FOREIGN":
		tok.tryKeyword("KEY")
		fk := &schema.ForeignKey{Name: constraintName}
		if tok.expect('(') {
			fk.Columns = splitIdentList(tok.balancedParens())
		}
		tok.tryKeyword("REFERENCES")
		fk.RefTable = tok.nextIdent()
		if tok.peekByte() == '(' {
			tok.expect('(')
			fk.RefColumns = splitIdentList(tok.balancedParens())
		}
		parseReferentialActions(tok, fk)
		t.ForeignKeys = append(t.ForeignKeys, fk)

	case "UNIQUE":
		if tok.expect('(') {
			cols := splitIdentList(tok.balancedParens())
			t.Unique = append(t.Unique, &schema.UniqueConstraint{
				Name:    constraintName,
				Columns: cols,
			})
		}
	}
}

// parseReferentialActions parses ON DELETE/UPDATE actions.
func parseReferentialActions(tok *tokenizer, fk *schema.ForeignKey) {
	for {
		word := tok.peek()
		if word == "" {
			break
		}
		if strings.ToUpper(word) != "ON" {
			// Could be MATCH, DEFERRABLE, etc. We skip those.
			tok.next()
			continue
		}
		tok.next() // consume ON

		actionType := strings.ToUpper(tok.next())
		action := parseAction(tok)

		switch actionType {
		case "DELETE":
			fk.OnDelete = action
		case "UPDATE":
			fk.OnUpdate = action
		}
	}
}

func parseAction(tok *tokenizer) schema.Action {
	word := strings.ToUpper(tok.next())
	switch word {
	case "CASCADE":
		return schema.ActionCascade
	case "RESTRICT":
		return schema.ActionRestrict
	case "SET":
		next := strings.ToUpper(tok.next())
		switch next {
		case "NULL":
			return schema.ActionSetNull
		case "DEFAULT":
			return schema.ActionSetDefault
		}
	case "NO":
		tok.tryKeyword("ACTION")
		return schema.ActionNoAction
	}
	return schema.ActionNone
}

// parseCreateIndex parses a CREATE [UNIQUE] INDEX statement.
func parseCreateIndex(stmt string, unique bool) (*schema.Index, string, error) {
	tok := newTokenizer(stmt)

	// CREATE [UNIQUE] INDEX
	tok.tryKeyword("CREATE")
	tok.tryKeyword("UNIQUE")
	tok.tryKeyword("INDEX")

	// Optional "IF NOT EXISTS"
	tok.tryKeywords("IF", "NOT", "EXISTS")

	idxName := tok.nextIdent()

	// ON table_name
	tok.tryKeyword("ON")
	tableName := tok.nextIdent()

	// (column_list)
	var cols []string
	if tok.expect('(') {
		content := tok.balancedParens()
		cols = splitIdentList(content)
	}

	return &schema.Index{
		Name:    idxName,
		Columns: cols,
		Unique:  unique,
	}, tableName, nil
}

// parseCreateView parses a CREATE VIEW statement.
func parseCreateView(stmt string) (*schema.View, error) {
	tok := newTokenizer(stmt)

	tok.tryKeyword("CREATE")
	tok.tryKeyword("VIEW")

	// Optional "IF NOT EXISTS"
	tok.tryKeywords("IF", "NOT", "EXISTS")

	name := tok.nextIdent()

	// AS <query>
	tok.tryKeyword("AS")

	query := strings.TrimSpace(tok.remaining())

	return &schema.View{
		Name:  name,
		Query: query,
	}, nil
}

// parseDefaultValue extracts the default value expression, handling parenthesized
// expressions and quoted strings.
func parseDefaultValue(tok *tokenizer) string {
	tok.skipWhitespace()

	if tok.peekByte() == '(' {
		tok.expect('(')
		return "(" + tok.balancedParens() + ")"
	}

	if tok.peekByte() == '\'' {
		return tok.nextQuotedString()
	}

	// Simple value (number, keyword like TRUE/FALSE/NULL, function call).
	val := tok.next()
	// Check if followed by parenthesized args (function call).
	if tok.peekByte() == '(' {
		tok.expect('(')
		val += "(" + tok.balancedParens() + ")"
	}
	return val
}

// normalizeType maps SQLite type names to canonical forms that align with
// the type mapper in gen/types.go.
func normalizeType(rawType string) string {
	upper := strings.ToUpper(strings.TrimSpace(rawType))

	if upper == "" {
		// SQLite allows columns with no type (defaults to BLOB affinity behavior,
		// but practically it's a dynamic type). Map to BLOB.
		return "blob"
	}

	// SQLite type affinity rules (https://www.sqlite.org/datatype3.html):
	// 1. If the type contains "INT", affinity is INTEGER.
	// 2. If the type contains "CHAR", "CLOB", or "TEXT", affinity is TEXT.
	// 3. If the type contains "BLOB" or is empty, affinity is BLOB (NONE).
	// 4. If the type contains "REAL", "FLOA", or "DOUB", affinity is REAL.
	// 5. Otherwise, affinity is NUMERIC.
	//
	// We map common exact types first, then fall back to affinity rules.

	if mapped, ok := sqliteTypeMap[upper]; ok {
		return mapped
	}

	// Affinity-based fallback.
	if strings.Contains(upper, "INT") {
		return "integer"
	}
	if strings.Contains(upper, "CHAR") || strings.Contains(upper, "CLOB") || strings.Contains(upper, "TEXT") {
		return "text"
	}
	if strings.Contains(upper, "BLOB") {
		return "blob"
	}
	if strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOA") || strings.Contains(upper, "DOUB") {
		return "double precision"
	}

	// NUMERIC affinity: includes DECIMAL, NUMERIC, BOOLEAN, DATE, DATETIME.
	return "numeric"
}

// sqliteTypeMap maps exact SQLite type strings to canonical forms.
var sqliteTypeMap = map[string]string{
	// Integer types
	"INTEGER":  "integer",
	"INT":      "integer",
	"TINYINT":  "integer",
	"SMALLINT": "integer",
	"MEDIUMINT": "integer",
	"BIGINT":   "bigint",
	"INT2":     "integer",
	"INT8":     "bigint",

	// Text types
	"TEXT":              "text",
	"CHARACTER":         "text",
	"VARCHAR":           "text",
	"VARYING CHARACTER": "text",
	"NCHAR":             "text",
	"NVARCHAR":          "text",
	"CLOB":              "text",

	// Blob
	"BLOB": "blob",

	// Real types (SQLite REAL is always 8-byte IEEE float)
	"REAL":             "double precision",
	"DOUBLE":           "double precision",
	"DOUBLE PRECISION": "double precision",
	"FLOAT":            "double precision",

	// Numeric types
	"NUMERIC": "numeric",
	"DECIMAL": "numeric",
	"BOOLEAN": "boolean",
	"DATE":    "date",
	"DATETIME": "datetime",

	// Timestamp (common in ORMs even though not official SQLite)
	"TIMESTAMP": "datetime",
}

// isConstraintKeyword returns true if the word starts a constraint clause
// (not a type name).
func isConstraintKeyword(upper string) bool {
	switch upper {
	case "PRIMARY", "NOT", "NULL", "UNIQUE", "CHECK", "DEFAULT",
		"REFERENCES", "AUTOINCREMENT", "COLLATE", "CONSTRAINT",
		"GENERATED", "ON":
		return true
	}
	return false
}

// splitTopLevel splits a string on commas, respecting nested parentheses.
func splitTopLevel(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inSingle {
			current.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					current.WriteByte(s[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			current.WriteByte(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
			current.WriteByte(ch)
		case '"':
			inDouble = true
			current.WriteByte(ch)
		case '(':
			depth++
			current.WriteByte(ch)
		case ')':
			depth--
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	last := strings.TrimSpace(current.String())
	if last != "" {
		parts = append(parts, last)
	}
	return parts
}

// extractParenList extracts a comma-separated list of identifiers from a
// string that contains parentheses, e.g., "PRIMARY KEY (a, b)" -> ["a", "b"].
func extractParenList(s string) []string {
	start := strings.IndexByte(s, '(')
	if start < 0 {
		return nil
	}
	end := strings.LastIndexByte(s, ')')
	if end < 0 {
		return nil
	}
	return splitIdentList(s[start+1 : end])
}

// splitIdentList splits "a, b, c" into individual identifier names,
// stripping quotes and whitespace.
func splitIdentList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = unquoteIdent(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// unquoteIdent removes surrounding double quotes or backticks from an identifier.
func unquoteIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') || (s[0] == '[' && s[len(s)-1] == ']') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// tokenizer is a simple tokenizer for SQLite DDL.
type tokenizer struct {
	input string
	pos   int
}

func newTokenizer(input string) *tokenizer {
	return &tokenizer{input: input}
}

func (t *tokenizer) skipWhitespace() {
	for t.pos < len(t.input) && (t.input[t.pos] == ' ' || t.input[t.pos] == '\t' || t.input[t.pos] == '\n' || t.input[t.pos] == '\r') {
		t.pos++
	}
}

func (t *tokenizer) peekByte() byte {
	t.skipWhitespace()
	if t.pos >= len(t.input) {
		return 0
	}
	return t.input[t.pos]
}

func (t *tokenizer) peek() string {
	saved := t.pos
	word := t.next()
	t.pos = saved
	return word
}

func (t *tokenizer) next() string {
	t.skipWhitespace()
	if t.pos >= len(t.input) {
		return ""
	}

	// Quoted identifier
	if t.input[t.pos] == '"' || t.input[t.pos] == '`' || t.input[t.pos] == '[' {
		return t.nextQuotedIdent()
	}

	// Single-quoted string
	if t.input[t.pos] == '\'' {
		return t.nextQuotedString()
	}

	// Regular word
	start := t.pos
	for t.pos < len(t.input) && !unicode.IsSpace(rune(t.input[t.pos])) && t.input[t.pos] != '(' && t.input[t.pos] != ')' && t.input[t.pos] != ',' && t.input[t.pos] != ';' {
		t.pos++
	}
	return t.input[start:t.pos]
}

// nextIdent reads the next token and strips quotes.
func (t *tokenizer) nextIdent() string {
	return unquoteIdent(t.next())
}

func (t *tokenizer) nextQuotedIdent() string {
	if t.pos >= len(t.input) {
		return ""
	}
	opener := t.input[t.pos]
	var closer byte
	switch opener {
	case '"':
		closer = '"'
	case '`':
		closer = '`'
	case '[':
		closer = ']'
	default:
		return t.next()
	}

	t.pos++ // skip opener
	start := t.pos
	for t.pos < len(t.input) && t.input[t.pos] != closer {
		t.pos++
	}
	result := t.input[start:t.pos]
	if t.pos < len(t.input) {
		t.pos++ // skip closer
	}
	return string(opener) + result + string(closer)
}

func (t *tokenizer) nextQuotedString() string {
	if t.pos >= len(t.input) || t.input[t.pos] != '\'' {
		return ""
	}
	t.pos++ // skip opening '
	var sb strings.Builder
	sb.WriteByte('\'')
	for t.pos < len(t.input) {
		ch := t.input[t.pos]
		t.pos++
		if ch == '\'' {
			// Escaped quote?
			if t.pos < len(t.input) && t.input[t.pos] == '\'' {
				sb.WriteByte('\'')
				sb.WriteByte('\'')
				t.pos++
			} else {
				sb.WriteByte('\'')
				return sb.String()
			}
		} else {
			sb.WriteByte(ch)
		}
	}
	sb.WriteByte('\'')
	return sb.String()
}

func (t *tokenizer) expect(ch byte) bool {
	t.skipWhitespace()
	if t.pos < len(t.input) && t.input[t.pos] == ch {
		t.pos++
		return true
	}
	return false
}

func (t *tokenizer) expectKeywords(keywords ...string) {
	for _, kw := range keywords {
		_ = kw
		t.next()
	}
}

func (t *tokenizer) tryKeyword(kw string) bool {
	saved := t.pos
	word := t.next()
	if strings.EqualFold(word, kw) {
		return true
	}
	t.pos = saved
	return false
}

func (t *tokenizer) tryKeywords(keywords ...string) bool {
	saved := t.pos
	for _, kw := range keywords {
		if !t.tryKeyword(kw) {
			t.pos = saved
			return false
		}
	}
	return true
}

// balancedParens reads content inside parentheses. Assumes the opening '('
// has already been consumed. Returns the content and consumes the closing ')'.
func (t *tokenizer) balancedParens() string {
	depth := 1
	start := t.pos
	inSingle := false
	inDouble := false

	for t.pos < len(t.input) && depth > 0 {
		ch := t.input[t.pos]

		if inSingle {
			if ch == '\'' {
				if t.pos+1 < len(t.input) && t.input[t.pos+1] == '\'' {
					t.pos++ // skip escaped quote
				} else {
					inSingle = false
				}
			}
			t.pos++
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			t.pos++
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				result := t.input[start:t.pos]
				t.pos++ // consume closing ')'
				return result
			}
		}
		t.pos++
	}

	return t.input[start:t.pos]
}

func (t *tokenizer) remaining() string {
	t.skipWhitespace()
	result := t.input[t.pos:]
	t.pos = len(t.input)
	return result
}
