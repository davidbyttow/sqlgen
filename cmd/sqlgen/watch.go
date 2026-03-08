package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/gen"
	"github.com/davidbyttow/sqlgen/schema"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch SQL files and regenerate on changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfg.Input.DSN != "" {
				return fmt.Errorf("watch mode isn't supported with DSN input; use `sqlgen generate` instead")
			}

			// Initial generation.
			if err := runGenerate(cmd.Context(), cfg); err != nil {
				log.Printf("Generation error: %v", err)
			}

			// Set up file watcher.
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("creating watcher: %w", err)
			}
			defer watcher.Close()

			// Watch all input paths.
			for _, p := range cfg.Input.Paths {
				if err := watchPath(watcher, p); err != nil {
					return fmt.Errorf("watching %s: %w", p, err)
				}
			}

			fmt.Println("Watching for changes...")

			// Debounce: wait for writes to settle before regenerating.
			var debounce *time.Timer

			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return nil
					}
					if !isSQLFile(event.Name) {
						continue
					}
					if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
						continue
					}

					// Reset debounce timer.
					if debounce != nil {
						debounce.Stop()
					}
					debounce = time.AfterFunc(200*time.Millisecond, func() {
						fmt.Printf("\nChange detected: %s\n", event.Name)
						if err := runGenerate(cmd.Context(), cfg); err != nil {
							log.Printf("Generation error: %v", err)
						} else {
							fmt.Println("Regeneration complete.")
						}
					})

				case err, ok := <-watcher.Errors:
					if !ok {
						return nil
					}
					log.Printf("Watcher error: %v", err)
				}
			}
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "sqlgen.yaml", "path to config file")
	return cmd
}

func runGenerate(ctx context.Context, cfg *config.Config) error {
	s, err := loadSchema(ctx, cfg)
	if err != nil {
		return err
	}
	schema.ResolveRelationships(s)

	fmt.Printf("Parsed %d tables, %d enums, %d views\n", len(s.Tables), len(s.Enums), len(s.Views))

	g := gen.NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	fmt.Printf("Generated code in %s/\n", cfg.Output.Dir)
	return nil
}

func watchPath(watcher *fsnotify.Watcher, p string) error {
	info, err := os.Stat(p)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return watcher.Add(filepath.Dir(p))
	}

	// Watch directory and subdirectories.
	return filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}

func isSQLFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".sql")
}

