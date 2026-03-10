package runtime

// Columns controls which columns are included in Insert/Update/Upsert operations.
// Use Whitelist or Blacklist to create one.
type Columns struct {
	mode int // 0=all, 1=whitelist, 2=blacklist
	set  map[string]bool
}

// Whitelist returns a Columns that includes only the named columns.
func Whitelist(cols ...string) Columns {
	s := make(map[string]bool, len(cols))
	for _, c := range cols {
		s[c] = true
	}
	return Columns{mode: 1, set: s}
}

// Blacklist returns a Columns that includes all columns except the named ones.
func Blacklist(cols ...string) Columns {
	s := make(map[string]bool, len(cols))
	for _, c := range cols {
		s[c] = true
	}
	return Columns{mode: 2, set: s}
}

// FilterColumns applies column filtering to parallel col/val slices.
// If no Columns is provided (or zero value), all columns pass through.
func FilterColumns(cols []string, vals []any, filter ...Columns) ([]string, []any) {
	if len(filter) == 0 || filter[0].mode == 0 {
		return cols, vals
	}

	f := filter[0]
	outCols := make([]string, 0, len(cols))
	outVals := make([]any, 0, len(cols))

	for i, col := range cols {
		switch f.mode {
		case 1: // whitelist
			if f.set[col] {
				outCols = append(outCols, col)
				outVals = append(outVals, vals[i])
			}
		case 2: // blacklist
			if !f.set[col] {
				outCols = append(outCols, col)
				outVals = append(outVals, vals[i])
			}
		}
	}

	return outCols, outVals
}
