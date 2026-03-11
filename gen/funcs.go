package gen

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/davidbyttow/sqlgen/internal/naming"
	"github.com/davidbyttow/sqlgen/schema"
)

// TemplateFuncs returns the template function map used by all templates.
func TemplateFuncs(mapper *TypeMapper) template.FuncMap {
	return template.FuncMap{
		// Naming
		"pascal":     naming.ToPascal,
		"camel":      naming.ToCamel,
		"snake":      naming.ToSnake,
		"plural":     naming.Pluralize,
		"singular":   naming.Singularize,
		"safeName":   naming.SafeGoName,
		"safeCamel": func(s string) string {
			return naming.SafeGoName(naming.ToCamel(s))
		},
		"safePascal": func(s string) string {
			return naming.SafeGoName(naming.ToPascal(s))
		},
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      strings.Title,

		// Type mapping
		"goType": func(col *schema.Column, tableName ...string) string {
			tn := ""
			if len(tableName) > 0 {
				tn = tableName[0]
			}
			return mapper.GoTypeForTable(col, tn).Name
		},
		"goTypeObj": func(col *schema.Column, tableName ...string) GoType {
			tn := ""
			if len(tableName) > 0 {
				tn = tableName[0]
			}
			return mapper.GoTypeForTable(col, tn)
		},
		// testZeroVal returns a valid zero value expression for a column's Go type, for use in generated tests.
		"testZeroVal": func(col *schema.Column) string {
			typeName := mapper.GoTypeFor(col).Name
			// Enum columns: use empty string cast to the enum type.
			if col.EnumName != "" {
				return typeName + `("")`
			}
			switch typeName {
			case "string":
				return `""`
			case "int16", "int32", "int64", "int", "float32", "float64":
				return "0"
			case "bool":
				return "false"
			default:
				if strings.HasPrefix(typeName, "*") {
					return "nil"
				}
				if strings.HasPrefix(typeName, "[]") {
					return "nil"
				}
				return typeName + "{}"
			}
		},

		// nullGoType returns a Null[T]-wrapped version of a column's Go type for preload scanning.
		// If the column is already nullable, it returns the same Null[T] type.
		// If non-nullable, wraps in runtime.Null[BaseType].
		"nullGoType": func(col *schema.Column) string {
			gt := mapper.GoTypeFor(col)
			if col.IsNullable {
				// Already a Null[T] type, use it directly.
				return gt.Name
			}
			return "runtime.Null[" + gt.Name + "]"
		},
		// baseGoType returns the inner type name (e.g., "string" for both "string" and "runtime.Null[string]").
		"baseGoType": func(col *schema.Column) string {
			gt := mapper.GoTypeFor(col)
			name := gt.Name
			if strings.HasPrefix(name, "runtime.Null[") && strings.HasSuffix(name, "]") {
				return name[len("runtime.Null[") : len(name)-1]
			}
			return name
		},

		// String helpers
		"join":     strings.Join,
		"contains": strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"replace":  strings.ReplaceAll,
		"quote":    func(s string) string { return `"` + s + `"` },

		// Schema helpers
		"isNullable": func(col *schema.Column) bool {
			return col.IsNullable
		},
		"isPrimaryKey": func(col *schema.Column, table *schema.Table) bool {
			if table.PrimaryKey == nil {
				return false
			}
			for _, c := range table.PrimaryKey.Columns {
				if c == col.Name {
					return true
				}
			}
			return false
		},
		"nonPKColumns": func(table *schema.Table) []*schema.Column {
			if table.PrimaryKey == nil {
				return table.Columns
			}
			pkSet := make(map[string]bool)
			for _, c := range table.PrimaryKey.Columns {
				pkSet[c] = true
			}
			var cols []*schema.Column
			for _, c := range table.Columns {
				if !pkSet[c.Name] {
					cols = append(cols, c)
				}
			}
			return cols
		},
		"autoIncrColumns": func(table *schema.Table) []*schema.Column {
			var cols []*schema.Column
			for _, c := range table.Columns {
				if c.IsAutoIncrement {
					cols = append(cols, c)
				}
			}
			return cols
		},
		"insertableColumns": func(table *schema.Table) []*schema.Column {
			var cols []*schema.Column
			for _, c := range table.Columns {
				if !c.IsAutoIncrement {
					cols = append(cols, c)
				}
			}
			return cols
		},
		"hasAutoIncrPK": func(table *schema.Table) bool {
			if table.PrimaryKey == nil {
				return false
			}
			for _, pkCol := range table.PrimaryKey.Columns {
				for _, c := range table.Columns {
					if c.Name == pkCol && c.IsAutoIncrement {
						return true
					}
				}
			}
			return false
		},
		"defaultedColumns": func(table *schema.Table) []*schema.Column {
			var cols []*schema.Column
			for _, c := range table.Columns {
				if c.HasDefault && !c.IsAutoIncrement {
					cols = append(cols, c)
				}
			}
			return cols
		},

		// Collection helpers
		"last": func(i, length int) bool {
			return i == length-1
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range n {
				s[i] = i
			}
			return s
		},

		// Relationship helpers
		"relBelongsTo":       func() schema.RelationType { return schema.RelBelongsTo },
		"relHasOne":          func() schema.RelationType { return schema.RelHasOne },
		"relHasMany":         func() schema.RelationType { return schema.RelHasMany },
		"relManyToMany":      func() schema.RelationType { return schema.RelManyToMany },
		"relPolymorphicOne":  func() schema.RelationType { return schema.RelPolymorphicOne },
		"relPolymorphicMany": func() schema.RelationType { return schema.RelPolymorphicMany },

		// findTable looks up a table by name from the full schema table list.
		"findTable": func(tables []*schema.Table, name string) *schema.Table {
			for _, t := range tables {
				if t.Name == name {
					return t
				}
			}
			return nil
		},

		// hasToOneRels returns true if the table has any BelongsTo or HasOne relationships.
		// PolymorphicOne is excluded because it can't be preloaded via LEFT JOIN.
		"hasToOneRels": func(table *schema.Table) bool {
			for _, r := range table.Relationships {
				if r.Type == schema.RelBelongsTo || r.Type == schema.RelHasOne {
					return true
				}
			}
			return false
		},

		// fkWrap wraps srcExpr if dst column is nullable but src column is not (or vice versa).
		// Returns the expression to use on the right side of an assignment.
		"fkWrap": func(dstTable *schema.Table, dstCol string, srcTable *schema.Table, srcCol string, srcExpr string) string {
			dst := dstTable.FindColumn(dstCol)
			src := srcTable.FindColumn(srcCol)
			if dst == nil || src == nil {
				return srcExpr
			}
			if dst.IsNullable && !src.IsNullable {
				return "runtime.NewNull(" + srcExpr + ")"
			}
			if !dst.IsNullable && src.IsNullable {
				return srcExpr + ".Val"
			}
			return srcExpr
		},

		// structTags builds the struct tag string for a column given the tag config.
		"structTags": func(col *schema.Column, tags map[string]string) string {
			return buildStructTags(col.Name, tags)
		},
		// relTags builds struct tags with "-" value for relationship fields.
		"relTags": func(tags map[string]string) string {
			return buildRelTags(tags)
		},

		// relFieldName generates a unique, descriptive field name for a relationship.
		"relFieldName": func(rel *schema.Relationship, table *schema.Table) string {
			isSelfRef := rel.ForeignTable == table.Name

			switch rel.Type {
			case schema.RelBelongsTo:
				if isSelfRef && len(rel.Columns) > 0 {
					// parent_id -> Parent
					col := rel.Columns[0]
					col = strings.TrimSuffix(col, "_id")
					return naming.ToPascal(naming.Singularize(col))
				}
				return naming.ToPascal(naming.Singularize(rel.ForeignTable))
			case schema.RelHasOne:
				if isSelfRef && rel.ForeignKey != nil && len(rel.ForeignKey.Columns) > 0 {
					col := rel.ForeignKey.Columns[0]
					col = strings.TrimSuffix(col, "_id")
					return naming.ToPascal(col) + "Of"
				}
				return naming.ToPascal(naming.Singularize(rel.ForeignTable))
			case schema.RelHasMany:
				if isSelfRef {
					// Children for self-referencing HasMany
					if rel.ForeignKey != nil && len(rel.ForeignKey.Columns) > 0 {
						col := rel.ForeignKey.Columns[0]
						col = strings.TrimSuffix(col, "_id")
						return naming.ToPascal(naming.Pluralize(col)) + "Inverse"
					}
					return naming.ToPascal(rel.ForeignTable) + "Children"
				}
				return naming.ToPascal(rel.ForeignTable)
			case schema.RelManyToMany:
				return naming.ToPascal(rel.ForeignTable)
			case schema.RelPolymorphicOne:
				// e.g., "commentable" -> "CommentableUser" using TypeValue
				return naming.ToPascal(naming.Singularize(rel.ForeignTable))
			case schema.RelPolymorphicMany:
				// e.g., "Comments" on User table where type = "User"
				return naming.ToPascal(rel.ForeignTable)
			}
			return naming.ToPascal(rel.ForeignTable)
		},
	}
}

// applyTagCasing applies the named casing to a column name.
func applyTagCasing(colName, casing string) string {
	switch casing {
	case "snake":
		return naming.ToSnake(colName)
	case "camel":
		return naming.ToCamel(colName)
	case "pascal":
		return naming.ToPascal(colName)
	default: // "none"
		return colName
	}
}

// buildStructTags builds a backtick-wrapped struct tag string from a tag config map.
func buildStructTags(colName string, tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, tag := range keys {
		casing := tags[tag]
		parts = append(parts, fmt.Sprintf(`%s:"%s"`, tag, applyTagCasing(colName, casing)))
	}
	return "`" + strings.Join(parts, " ") + "`"
}

// buildRelTags builds struct tags with "-" for all configured tag names.
func buildRelTags(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, tag := range keys {
		parts = append(parts, fmt.Sprintf(`%s:"-"`, tag))
	}
	return "`" + strings.Join(parts, " ") + "`"
}
