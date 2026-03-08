package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "sqlgen",
		Short: "Generate type-safe Go code from database schemas",
		Long:  "sqlgen reads SQL DDL files and generates type-safe Go models, CRUD operations, and query builders.",
	}

	root.AddCommand(newGenerateCmd())
	root.AddCommand(newWatchCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
