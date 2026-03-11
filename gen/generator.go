package gen

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/internal/naming"
	"github.com/davidbyttow/sqlgen/schema"

	"golang.org/x/tools/imports"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

const runtimePkg = "github.com/davidbyttow/sqlgen/runtime"

// Generator produces Go source files from a schema.
type Generator struct {
	cfg    *config.Config
	schema *schema.Schema
	mapper *TypeMapper
	funcs  template.FuncMap
}

// NewGenerator creates a generator from config and schema.
func NewGenerator(cfg *config.Config, s *schema.Schema) *Generator {
	// Ensure Tags has defaults even if config wasn't loaded via Parse/Validate.
	if len(cfg.Tags) == 0 {
		cfg.Tags = map[string]string{"json": "snake", "db": "none"}
	}
	mapper := NewTypeMapper(cfg, runtimePkg)
	funcs := TemplateFuncs(mapper)

	// Add findCol helper that needs schema context.
	funcs["findCol"] = func(table *schema.Table, colName string) *schema.Column {
		col := table.FindColumn(colName)
		if col == nil {
			// Return a fallback to avoid template panics.
			return &schema.Column{Name: colName, DBType: "text"}
		}
		return col
	}

	return &Generator{
		cfg:    cfg,
		schema: s,
		mapper: mapper,
		funcs:  funcs,
	}
}

// Run generates all files and writes them to the output directory.
func (g *Generator) Run() error {
	outDir := g.cfg.Output.Dir
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	pkg := g.cfg.Output.Package
	if pkg == "" {
		pkg = filepath.Base(outDir)
	}

	// Track generated files for stale cleanup.
	generated := map[string]bool{}

	// Generate singleton dialect file.
	if err := g.generateSingleton("dialect.go.tmpl", outDir, pkg, "sqlgen_dialect.go", generated); err != nil {
		return err
	}

	// Generate constraints file if any table has named constraints.
	constraints := g.collectConstraints()
	if len(constraints) > 0 {
		constraintData := map[string]any{
			"Package":     pkg,
			"Constraints": constraints,
		}
		if err := g.generateFile("constraints.go.tmpl", constraintData, outDir, "sqlgen_constraints.go", generated); err != nil {
			return fmt.Errorf("generating constraints: %w", err)
		}
	}

	// Generate enum files.
	for _, enum := range g.schema.Enums {
		data := map[string]any{
			"Package": pkg,
			"Enum":    enum,
		}
		filename := fmt.Sprintf("sqlgen_enum_%s.go", naming.ToSnake(enum.Name))
		if err := g.generateFile("enum.go.tmpl", data, outDir, filename, generated); err != nil {
			return fmt.Errorf("generating enum %s: %w", enum.Name, err)
		}
		if g.cfg.Output.Tests {
			testFilename := fmt.Sprintf("sqlgen_enum_%s_test.go", naming.ToSnake(enum.Name))
			if err := g.generateFile("enum_test.go.tmpl", data, outDir, testFilename, generated); err != nil {
				return fmt.Errorf("generating enum test %s: %w", enum.Name, err)
			}
		}
	}

	// Generate per-table files.
	for _, table := range g.schema.Tables {
		if tc, ok := g.cfg.Tables[table.Name]; ok && tc.Skip {
			continue
		}

		tableImports := g.collectTableImports(table)

		data := map[string]any{
			"Package": pkg,
			"Table":   table,
			"Imports": tableImports.FormatBlock(),
			"Tags":    g.cfg.Tags,
		}

		snakeName := naming.ToSnake(table.Name)

		// Model
		if err := g.generateFile("model.go.tmpl", data, outDir, fmt.Sprintf("sqlgen_%s_model.go", snakeName), generated); err != nil {
			return fmt.Errorf("generating model for %s: %w", table.Name, err)
		}

		// CRUD
		crudImports := g.collectCRUDImports(table)
		ts := g.detectTimestamps(table)
		if ts.CreatedAt != "" || ts.UpdatedAt != "" {
			crudImports.Add("time")
		}
		crudData := map[string]any{
			"Package":    pkg,
			"Table":      table,
			"Imports":    crudImports.FormatBlock(),
			"NoHooks":    g.cfg.Output.NoHooks,
			"Timestamps": ts,
		}
		if err := g.generateFile("crud.go.tmpl", crudData, outDir, fmt.Sprintf("sqlgen_%s_crud.go", snakeName), generated); err != nil {
			return fmt.Errorf("generating crud for %s: %w", table.Name, err)
		}

		// Hooks (skip when disabled)
		if !g.cfg.Output.NoHooks {
			hooksData := map[string]any{
				"Package": pkg,
				"Table":   table,
			}
			if err := g.generateFile("hooks.go.tmpl", hooksData, outDir, fmt.Sprintf("sqlgen_%s_hooks.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating hooks for %s: %w", table.Name, err)
			}
		}

		// Where clauses
		whereImports := NewImportSet()
		whereImports.Add(runtimePkg)
		whereData := map[string]any{
			"Package": pkg,
			"Table":   table,
			"Imports": whereImports.FormatBlock(),
		}
		if err := g.generateFile("where.go.tmpl", whereData, outDir, fmt.Sprintf("sqlgen_%s_where.go", snakeName), generated); err != nil {
			return fmt.Errorf("generating where for %s: %w", table.Name, err)
		}

		// Loaders (only for tables with relationships)
		if len(table.Relationships) > 0 {
			loaderImports := g.collectLoaderImports()
			loaderData := map[string]any{
				"Package":   pkg,
				"Table":     table,
				"Imports":   loaderImports.FormatBlock(),
				"AllTables": g.schema.Tables,
			}
			if err := g.generateFile("loaders.go.tmpl", loaderData, outDir, fmt.Sprintf("sqlgen_%s_loaders.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating loaders for %s: %w", table.Name, err)
			}
		}

		// Relationship mutations (only for tables with relationships)
		if len(table.Relationships) > 0 {
			relImports := NewImportSet()
			relImports.Add("context")
			relImports.Add("fmt")
			relImports.Add(runtimePkg)
			relData := map[string]any{
				"Package":   pkg,
				"Table":     table,
				"Imports":   relImports.FormatBlock(),
				"AllTables": g.schema.Tables,
			}
			if err := g.generateFile("relations.go.tmpl", relData, outDir, fmt.Sprintf("sqlgen_%s_relations.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating relations for %s: %w", table.Name, err)
			}
		}

		// Count loaders (only for tables with HasMany or ManyToMany)
		if g.hasCountableRels(table) {
			countImports := NewImportSet()
			countImports.Add("context")
			countImports.Add("fmt")
			countImports.Add(runtimePkg)
			countData := map[string]any{
				"Package":   pkg,
				"Table":     table,
				"Imports":   countImports.FormatBlock(),
				"AllTables": g.schema.Tables,
			}
			if err := g.generateFile("count_loaders.go.tmpl", countData, outDir, fmt.Sprintf("sqlgen_%s_count_loaders.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating count loaders for %s: %w", table.Name, err)
			}
		}

		// Preloads (only for tables with to-one relationships)
		if g.hasToOneRels(table) {
			preloadImports := g.collectPreloadImports(table)
			preloadData := map[string]any{
				"Package":   pkg,
				"Table":     table,
				"Imports":   preloadImports.FormatBlock(),
				"AllTables": g.schema.Tables,
			}
			if err := g.generateFile("preload.go.tmpl", preloadData, outDir, fmt.Sprintf("sqlgen_%s_preload.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating preload for %s: %w", table.Name, err)
			}
		}

		// Tests (opt-in)
		if g.cfg.Output.Tests {
			testData := map[string]any{
				"Package": pkg,
				"Table":   table,
			}
			if err := g.generateFile("test.go.tmpl", testData, outDir, fmt.Sprintf("sqlgen_%s_test.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating tests for %s: %w", table.Name, err)
			}
		}

		// Factories (opt-in)
		if g.cfg.Output.Factories {
			factoryImports := g.collectFactoryImports(table)
			factoryData := map[string]any{
				"Package": pkg,
				"Table":   table,
				"Imports": factoryImports.FormatBlock(),
				"Enums":   g.schema.Enums,
			}
			if err := g.generateFile("factory.go.tmpl", factoryData, outDir, fmt.Sprintf("sqlgen_%s_factory.go", snakeName), generated); err != nil {
				return fmt.Errorf("generating factory for %s: %w", table.Name, err)
			}
		}

		// Extra user templates
		if err := g.renderExtraTemplates(table, tableImports, outDir, pkg, snakeName, generated); err != nil {
			return err
		}
	}

	// Stale file cleanup.
	return g.cleanStaleFiles(outDir, generated)
}

func (g *Generator) generateSingleton(tmplName, outDir, pkg, filename string, generated map[string]bool) error {
	data := map[string]any{
		"Package": pkg,
	}
	return g.generateFile(tmplName, data, outDir, filename, generated)
}

// builtinTemplates is the set of template names shipped with sqlgen.
var builtinTemplates = map[string]bool{
	"constraints.go.tmpl":   true,
	"factory.go.tmpl":       true,
	"model.go.tmpl":         true,
	"crud.go.tmpl":          true,
	"hooks.go.tmpl":         true,
	"where.go.tmpl":         true,
	"loaders.go.tmpl":       true,
	"relations.go.tmpl":     true,
	"count_loaders.go.tmpl": true,
	"preload.go.tmpl":       true,
	"test.go.tmpl":          true,
	"enum.go.tmpl":          true,
	"enum_test.go.tmpl":     true,
	"dialect.go.tmpl":       true,
}

// readTemplate reads a template, checking the user's override directory first.
func (g *Generator) readTemplate(tmplName string) ([]byte, error) {
	if g.cfg.Output.Templates != "" {
		userPath := filepath.Join(g.cfg.Output.Templates, tmplName)
		if data, err := os.ReadFile(userPath); err == nil {
			return data, nil
		}
	}
	return templateFS.ReadFile("templates/" + tmplName)
}

func (g *Generator) generateFile(tmplName string, data map[string]any, outDir, filename string, generated map[string]bool) error {
	tmplContent, err := g.readTemplate(tmplName)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", tmplName, err)
	}

	tmpl, err := template.New(tmplName).Funcs(g.funcs).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parsing template %s: %w", tmplName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template %s: %w", tmplName, err)
	}

	// Run goimports to fix formatting and imports.
	formatted, err := imports.Process(filename, buf.Bytes(), &imports.Options{
		Comments:  true,
		TabIndent: true,
		TabWidth:  8,
	})
	if err != nil {
		// Write unformatted for debugging.
		outPath := filepath.Join(outDir, filename)
		os.WriteFile(outPath, buf.Bytes(), 0o644)
		return fmt.Errorf("formatting %s: %w (unformatted file written for debugging)", filename, err)
	}

	outPath := filepath.Join(outDir, filename)
	if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	generated[filename] = true
	return nil
}

func (g *Generator) collectTableImports(table *schema.Table) *ImportSet {
	imports := NewImportSet()
	imports.Add(runtimePkg)

	for _, col := range table.Columns {
		gt := g.mapper.GoTypeForTable(col, table.Name)
		imports.AddGoType(gt)
	}

	return imports
}

func (g *Generator) collectCRUDImports(table *schema.Table) *ImportSet {
	imports := NewImportSet()
	imports.Add("context")
	imports.Add(runtimePkg)

	for _, col := range table.Columns {
		gt := g.mapper.GoTypeForTable(col, table.Name)
		imports.AddGoType(gt)
	}

	return imports
}

const fakePkg = "github.com/davidbyttow/sqlgen/runtime/fake"

func (g *Generator) collectFactoryImports(table *schema.Table) *ImportSet {
	imports := NewImportSet()
	imports.Add("context")
	imports.Add(runtimePkg)
	imports.Add(fakePkg)

	for _, col := range table.Columns {
		gt := g.mapper.GoTypeForTable(col, table.Name)
		imports.AddGoType(gt)
	}

	return imports
}

func (g *Generator) collectLoaderImports() *ImportSet {
	imports := NewImportSet()
	imports.Add("context")
	imports.Add("fmt")
	imports.Add(runtimePkg)
	return imports
}

// ConstraintEntry is a deduplicated constraint for template rendering.
type ConstraintEntry struct {
	GoName string // e.g., "UserEmailKey"
	DBName string // e.g., "users_email_key"
}

func (g *Generator) collectConstraints() []ConstraintEntry {
	seen := map[string]bool{}
	var entries []ConstraintEntry

	add := func(tableName, constraintName string) {
		if constraintName == "" {
			return
		}
		goName := g.constraintConst(tableName, constraintName)
		if seen[goName] {
			return
		}
		seen[goName] = true
		entries = append(entries, ConstraintEntry{GoName: goName, DBName: constraintName})
	}

	for _, t := range g.schema.Tables {
		if tc, ok := g.cfg.Tables[t.Name]; ok && tc.Skip {
			continue
		}
		if t.PrimaryKey != nil {
			add(t.Name, t.PrimaryKey.Name)
		}
		for _, u := range t.Unique {
			add(t.Name, u.Name)
		}
		for _, fk := range t.ForeignKeys {
			add(t.Name, fk.Name)
		}
		for _, idx := range t.Indexes {
			if idx.Unique {
				add(t.Name, idx.Name)
			}
		}
	}
	return entries
}

func (g *Generator) constraintConst(tableName, constraintName string) string {
	structName := naming.ToPascal(naming.Singularize(tableName))
	clean := constraintName
	if strings.HasPrefix(clean, tableName+"_") {
		clean = clean[len(tableName)+1:]
	}
	return structName + naming.ToPascal(clean)
}


func (g *Generator) hasToOneRels(table *schema.Table) bool {
	for _, r := range table.Relationships {
		if r.Type == schema.RelBelongsTo || r.Type == schema.RelHasOne {
			return true
		}
	}
	return false
}

func (g *Generator) hasCountableRels(table *schema.Table) bool {
	for _, r := range table.Relationships {
		if r.Type == schema.RelHasMany || r.Type == schema.RelManyToMany || r.Type == schema.RelPolymorphicMany {
			return true
		}
	}
	return false
}

func (g *Generator) collectPreloadImports(table *schema.Table) *ImportSet {
	imports := NewImportSet()
	imports.Add(runtimePkg)

	// Add imports for all foreign table column types.
	for _, rel := range table.Relationships {
		if rel.Type != schema.RelBelongsTo && rel.Type != schema.RelHasOne {
			continue
		}
		foreignTable := g.schema.FindTable(rel.ForeignTable)
		if foreignTable == nil {
			continue
		}
		for _, col := range foreignTable.Columns {
			gt := g.mapper.GoTypeForTable(col, foreignTable.Name)
			imports.AddGoType(gt)
		}
	}

	return imports
}

// TimestampData holds per-table timestamp info for template rendering.
type TimestampData struct {
	CreatedAt string // Column name (empty if not found or disabled)
	UpdatedAt string // Column name (empty if not found or disabled)
	NowExpr   string // Go expression for current time (e.g., "time.Now()")
}

// detectTimestamps checks if a table has the configured timestamp columns.
func (g *Generator) detectTimestamps(table *schema.Table) TimestampData {
	td := TimestampData{NowExpr: "time.Now()"}

	if name := g.cfg.Timestamps.CreatedAt; name != "-" {
		if table.FindColumn(name) != nil {
			td.CreatedAt = name
		}
	}
	if name := g.cfg.Timestamps.UpdatedAt; name != "-" {
		if table.FindColumn(name) != nil {
			td.UpdatedAt = name
		}
	}

	return td
}

// renderExtraTemplates renders any user-provided templates that aren't built-in overrides.
func (g *Generator) renderExtraTemplates(table *schema.Table, tableImports *ImportSet, outDir, pkg, snakeName string, generated map[string]bool) error {
	if g.cfg.Output.Templates == "" {
		return nil
	}
	entries, err := os.ReadDir(g.cfg.Output.Templates)
	if err != nil {
		return fmt.Errorf("reading templates dir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go.tmpl") || builtinTemplates[name] {
			continue
		}
		// Derive output filename: "foo.go.tmpl" -> "sqlgen_{table}_foo.go"
		base := strings.TrimSuffix(name, ".tmpl")
		outFilename := fmt.Sprintf("sqlgen_%s_%s", snakeName, base)

		data := map[string]any{
			"Package":   pkg,
			"Table":     table,
			"Imports":   tableImports.FormatBlock(),
			"Tags":      g.cfg.Tags,
			"AllTables": g.schema.Tables,
		}
		if err := g.generateFile(name, data, outDir, outFilename, generated); err != nil {
			return fmt.Errorf("generating extra template %s for %s: %w", name, table.Name, err)
		}
	}
	return nil
}

// cleanStaleFiles removes generated files that weren't produced in this run.
func (g *Generator) cleanStaleFiles(outDir string, generated map[string]bool) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil // Dir might not exist yet
	}

	for _, entry := range entries {
		name := entry.Name()
		// Only clean files with our prefix.
		if !strings.HasPrefix(name, "sqlgen_") || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !generated[name] {
			path := filepath.Join(outDir, name)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("removing stale file %s: %w", path, err)
			}
		}
	}
	return nil
}
