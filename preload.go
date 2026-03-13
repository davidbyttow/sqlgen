package sqlgen

// PreloadDef describes how to LEFT JOIN a related table for preloading.
// Generated code creates these per-relationship.
type PreloadDef struct {
	Name     string   // Relationship name (e.g., "Organization")
	Table    string   // Related table name
	JoinCond string   // ON clause (e.g., "organizations"."id" = "users"."org_id")
	Columns  []string // Aliased SELECT expressions (e.g., "organizations"."id" AS __p_Organization_id)
}

// Preload adds a LEFT JOIN eager load for a to-one relationship.
// The PreloadDef is generated per-relationship on each model.
func Preload(def PreloadDef) QueryMod {
	return func(q *Query) {
		q.preloads = append(q.preloads, def)
	}
}
