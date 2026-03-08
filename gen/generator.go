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
		}

		snakeName := naming.ToSnake(table.Name)

		// Model
		if err := g.generateFile("model.go.tmpl", data, outDir, fmt.Sprintf("sqlgen_%s_model.go", snakeName), generated); err != nil {
			return fmt.Errorf("generating model for %s: %w", table.Name, err)
		}

		// CRUD
		crudImports := g.collectCRUDImports(table)
		crudData := map[string]any{
			"Package": pkg,
			"Table":   table,
			"Imports": crudImports.FormatBlock(),
		}
		if err := g.generateFile("crud.go.tmpl", crudData, outDir, fmt.Sprintf("sqlgen_%s_crud.go", snakeName), generated); err != nil {
			return fmt.Errorf("generating crud for %s: %w", table.Name, err)
		}

		// Hooks
		hooksData := map[string]any{
			"Package": pkg,
			"Table":   table,
		}
		if err := g.generateFile("hooks.go.tmpl", hooksData, outDir, fmt.Sprintf("sqlgen_%s_hooks.go", snakeName), generated); err != nil {
			return fmt.Errorf("generating hooks for %s: %w", table.Name, err)
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

func (g *Generator) generateFile(tmplName string, data map[string]any, outDir, filename string, generated map[string]bool) error {
	tmplContent, err := templateFS.ReadFile("templates/" + tmplName)
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
		gt := g.mapper.GoTypeFor(col)
		imports.AddGoType(gt)
	}

	return imports
}

func (g *Generator) collectCRUDImports(table *schema.Table) *ImportSet {
	imports := NewImportSet()
	imports.Add("context")
	imports.Add(runtimePkg)

	for _, col := range table.Columns {
		gt := g.mapper.GoTypeFor(col)
		imports.AddGoType(gt)
	}

	return imports
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
