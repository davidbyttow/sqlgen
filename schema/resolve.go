package schema

// ResolveRelationships walks all foreign keys in the schema and infers
// relationships between tables. It populates the Relationships field on each table.
//
// It detects:
//   - BelongsTo: table has a FK to another table (many-to-one)
//   - HasOne: another table has a FK to this table, and the FK columns are unique
//   - HasMany: another table has a FK to this table (one-to-many)
//   - ManyToMany: a join table connects two tables via two FKs
func ResolveRelationships(s *Schema) {
	tableMap := make(map[string]*Table, len(s.Tables))
	for _, t := range s.Tables {
		tableMap[t.Name] = t
		if t.Schema != "" {
			tableMap[t.Schema+"."+t.Name] = t
		}
	}

	// Detect join tables: tables whose PK spans exactly two FKs.
	joinTables := map[string]bool{}
	for _, t := range s.Tables {
		if isJoinTable(t) {
			joinTables[t.Name] = true
		}
	}

	for _, t := range s.Tables {
		if joinTables[t.Name] {
			buildManyToMany(t, tableMap)
			continue
		}

		for _, fk := range t.ForeignKeys {
			refKey := fk.RefTable
			if fk.RefSchema != "" {
				refKey = fk.RefSchema + "." + fk.RefTable
			}
			refTable, ok := tableMap[refKey]
			if !ok {
				continue
			}

			// This table belongs to the referenced table.
			t.Relationships = append(t.Relationships, &Relationship{
				Type:           RelBelongsTo,
				Table:          t.Name,
				Columns:        fk.Columns,
				ForeignTable:   refTable.Name,
				ForeignColumns: fk.RefColumns,
				ForeignKey:     fk,
			})

			// The referenced table has-one or has-many back to this table.
			relType := RelHasMany
			if fkColumnsAreUnique(t, fk.Columns) {
				relType = RelHasOne
			}
			refTable.Relationships = append(refTable.Relationships, &Relationship{
				Type:           relType,
				Table:          refTable.Name,
				Columns:        fk.RefColumns,
				ForeignTable:   t.Name,
				ForeignColumns: fk.Columns,
				ForeignKey:     fk,
			})
		}
	}
}

// isJoinTable returns true if a table looks like a many-to-many join table:
// - Has exactly 2 foreign keys
// - Has a composite primary key
// - PK columns are exactly the union of the two FK column sets
func isJoinTable(t *Table) bool {
	if len(t.ForeignKeys) != 2 {
		return false
	}
	if t.PrimaryKey == nil || len(t.PrimaryKey.Columns) < 2 {
		return false
	}

	// Collect all FK columns.
	fkCols := map[string]bool{}
	for _, fk := range t.ForeignKeys {
		for _, c := range fk.Columns {
			fkCols[c] = true
		}
	}

	// PK columns must be exactly the FK columns.
	if len(t.PrimaryKey.Columns) != len(fkCols) {
		return false
	}
	for _, c := range t.PrimaryKey.Columns {
		if !fkCols[c] {
			return false
		}
	}
	return true
}

func buildManyToMany(joinTable *Table, tableMap map[string]*Table) {
	if len(joinTable.ForeignKeys) != 2 {
		return
	}

	fk0 := joinTable.ForeignKeys[0]
	fk1 := joinTable.ForeignKeys[1]

	refKey0 := fk0.RefTable
	if fk0.RefSchema != "" {
		refKey0 = fk0.RefSchema + "." + fk0.RefTable
	}
	refKey1 := fk1.RefTable
	if fk1.RefSchema != "" {
		refKey1 = fk1.RefSchema + "." + fk1.RefTable
	}

	t0 := tableMap[refKey0]
	t1 := tableMap[refKey1]
	if t0 == nil || t1 == nil {
		return
	}

	// t0 has many t1 through joinTable.
	t0.Relationships = append(t0.Relationships, &Relationship{
		Type:               RelManyToMany,
		Table:              t0.Name,
		Columns:            fk0.RefColumns,
		ForeignTable:       t1.Name,
		ForeignColumns:     fk1.RefColumns,
		JoinTable:          joinTable.Name,
		JoinLocalColumns:   fk0.Columns,
		JoinForeignColumns: fk1.Columns,
	})

	// t1 has many t0 through joinTable.
	t1.Relationships = append(t1.Relationships, &Relationship{
		Type:               RelManyToMany,
		Table:              t1.Name,
		Columns:            fk1.RefColumns,
		ForeignTable:       t0.Name,
		ForeignColumns:     fk0.RefColumns,
		JoinTable:          joinTable.Name,
		JoinLocalColumns:   fk1.Columns,
		JoinForeignColumns: fk0.Columns,
	})
}

// PolymorphicDef describes a polymorphic relationship from config.
type PolymorphicDef struct {
	Table      string            // Table with type+id columns
	TypeColumn string            // Column holding type discriminator
	IDColumn   string            // Column holding FK value
	Targets    map[string]string // TypeValue -> target table name
}

// ResolvePolymorphic adds polymorphic relationships to the schema.
// For each polymorphic definition, it creates:
// - RelPolymorphicOne on the source table (one per target)
// - RelPolymorphicMany on each target table
func ResolvePolymorphic(s *Schema, defs []PolymorphicDef) {
	tableMap := make(map[string]*Table, len(s.Tables))
	for _, t := range s.Tables {
		tableMap[t.Name] = t
	}

	for _, def := range defs {
		srcTable := tableMap[def.Table]
		if srcTable == nil {
			continue
		}

		for typeVal, targetName := range def.Targets {
			targetTable := tableMap[targetName]
			if targetTable == nil {
				continue
			}

			// Find target PK column
			if targetTable.PrimaryKey == nil || len(targetTable.PrimaryKey.Columns) == 0 {
				continue
			}
			targetPK := targetTable.PrimaryKey.Columns[0]

			// Source table "belongs to" target via polymorphic
			srcTable.Relationships = append(srcTable.Relationships, &Relationship{
				Type:           RelPolymorphicOne,
				Table:          srcTable.Name,
				Columns:        []string{def.IDColumn},
				ForeignTable:   targetName,
				ForeignColumns: []string{targetPK},
				TypeColumn:     def.TypeColumn,
				IDColumn:       def.IDColumn,
				TypeValue:      typeVal,
			})

			// Target table "has many" source via polymorphic
			targetTable.Relationships = append(targetTable.Relationships, &Relationship{
				Type:           RelPolymorphicMany,
				Table:          targetName,
				Columns:        []string{targetPK},
				ForeignTable:   srcTable.Name,
				ForeignColumns: []string{def.IDColumn},
				TypeColumn:     def.TypeColumn,
				IDColumn:       def.IDColumn,
				TypeValue:      typeVal,
			})
		}
	}
}

// fkColumnsAreUnique checks if the FK columns are covered by a unique constraint or unique index.
func fkColumnsAreUnique(t *Table, cols []string) bool {
	// Single-column FK that IS the primary key.
	if t.PrimaryKey != nil && columnsMatch(t.PrimaryKey.Columns, cols) {
		return true
	}

	// Check unique constraints.
	for _, uc := range t.Unique {
		if columnsMatch(uc.Columns, cols) {
			return true
		}
	}

	return false
}

// columnsMatch returns true if both slices contain the same column names (order-independent).
func columnsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, c := range a {
		set[c] = true
	}
	for _, c := range b {
		if !set[c] {
			return false
		}
	}
	return true
}
