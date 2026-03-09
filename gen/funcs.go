package gen

import (
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
		"goType": func(col *schema.Column) string {
			return mapper.GoTypeFor(col).Name
		},
		"goTypeObj": func(col *schema.Column) GoType {
			return mapper.GoTypeFor(col)
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
		"relBelongsTo":  func() schema.RelationType { return schema.RelBelongsTo },
		"relHasOne":     func() schema.RelationType { return schema.RelHasOne },
		"relHasMany":    func() schema.RelationType { return schema.RelHasMany },
		"relManyToMany": func() schema.RelationType { return schema.RelManyToMany },

		// findTable looks up a table by name from the full schema table list.
		"findTable": func(tables []*schema.Table, name string) *schema.Table {
			for _, t := range tables {
				if t.Name == name {
					return t
				}
			}
			return nil
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
			}
			return naming.ToPascal(rel.ForeignTable)
		},
	}
}
