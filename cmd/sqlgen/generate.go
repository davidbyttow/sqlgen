package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/gen"
	"github.com/davidbyttow/sqlgen/schema"
	"github.com/davidbyttow/sqlgen/schema/postgres"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from SQL schema files",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			s, err := loadSchema(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			// Resolve relationships.
			schema.ResolveRelationships(s)

			// Resolve polymorphic relationships from config.
			if len(cfg.Polymorphic) > 0 {
				var defs []schema.PolymorphicDef
				for _, p := range cfg.Polymorphic {
					defs = append(defs, schema.PolymorphicDef{
						Table:      p.Table,
						TypeColumn: p.TypeColumn,
						IDColumn:   p.IDColumn,
						Targets:    p.Targets,
					})
				}
				schema.ResolvePolymorphic(s, defs)
			}

			fmt.Printf("Parsed %d tables, %d enums, %d views\n", len(s.Tables), len(s.Enums), len(s.Views))

			// Generate code.
			g := gen.NewGenerator(cfg, s)
			if err := g.Run(); err != nil {
				return fmt.Errorf("generation failed: %w", err)
			}

			fmt.Printf("Generated code in %s/\n", cfg.Output.Dir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "sqlgen.yaml", "path to config file")

	return cmd
}

// loadSchema builds the Schema IR from either DDL files or a live database,
// depending on the config.
func loadSchema(ctx context.Context, cfg *config.Config) (*schema.Schema, error) {
	if cfg.Input.DSN != "" {
		return postgres.Introspect(ctx, cfg.Input.DSN)
	}
	parser := getParser(cfg.Input.Dialect)
	return parseInputs(parser, cfg.Input.Paths)
}

func getParser(dialect string) schema.Parser {
	switch dialect {
	case "postgres":
		return &postgres.Parser{}
	default:
		return &postgres.Parser{} // Default, validated in config.
	}
}

func parseInputs(parser schema.Parser, paths []string) (*schema.Schema, error) {
	merged := &schema.Schema{}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("input path %q: %w", p, err)
		}

		var s *schema.Schema
		if info.IsDir() {
			s, err = parser.ParseDir(p)
		} else if filepath.Ext(p) == ".sql" {
			s, err = parser.ParseFile(p)
		} else {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}

		merged.Tables = append(merged.Tables, s.Tables...)
		merged.Enums = append(merged.Enums, s.Enums...)
		merged.Views = append(merged.Views, s.Views...)
	}

	return merged, nil
}
