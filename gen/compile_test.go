package gen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// sharedModDir is a temp directory with a go.mod already tidied.
// Set up once in TestMain, used by all tests that need to compile generated code.
var sharedModDir string

func TestMain(m *testing.M) {
	// Create a shared module directory for compilation tests.
	dir, err := os.MkdirTemp("", "sqlgen-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}
	sharedModDir = dir

	goMod := `module testgen

go 1.23

require github.com/davidbyttow/sqlgen v0.0.0

replace github.com/davidbyttow/sqlgen => ` + getModuleRoot() + `
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing go.mod: %v\n", err)
		os.Exit(1)
	}

	// Write a minimal Go file so go mod tidy has something to resolve.
	pkgDir := filepath.Join(dir, "stub")
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, "stub.go"), []byte(`package stub

import _ "github.com/davidbyttow/sqlgen/runtime"
`), 0o644)

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go mod tidy: %s\n%v\n", out, err)
		os.Exit(1)
	}

	// Warm the build cache with a single compile.
	warmCmd := exec.Command("go", "build", "./stub/...")
	warmCmd.Dir = dir
	warmCmd.Env = append(os.Environ(), "GOFLAGS=")
	warmCmd.CombinedOutput() // Ignore errors; just warming the cache.

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

// generateAndBuild generates code into a sub-package of the shared module and runs go build.
// Returns the output directory path. Fails the test on error.
func generateAndBuild(t *testing.T, g *Generator, pkgName string) string {
	t.Helper()
	outDir := filepath.Join(sharedModDir, pkgName)
	g.cfg.Output.Dir = outDir
	g.cfg.Output.Package = pkgName

	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	cmd := exec.Command("go", "build", fmt.Sprintf("./%s/...", pkgName))
	cmd.Dir = sharedModDir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		dumpGeneratedFiles(t, outDir)
		t.Fatalf("generated code does not compile:\n%s\n%v", out, err)
	}
	return outDir
}

// generateAndTest generates code and runs go test on it.
func generateAndTest(t *testing.T, g *Generator, pkgName string) string {
	t.Helper()
	outDir := filepath.Join(sharedModDir, pkgName)
	g.cfg.Output.Dir = outDir
	g.cfg.Output.Package = pkgName

	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	cmd := exec.Command("go", "test", fmt.Sprintf("./%s/...", pkgName), "-v")
	cmd.Dir = sharedModDir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		dumpGeneratedFiles(t, outDir)
		t.Fatalf("generated tests failed:\n%s\n%v", out, err)
	}
	t.Logf("Test output:\n%s", out)
	return outDir
}

func dumpGeneratedFiles(t *testing.T, outDir string) {
	t.Helper()
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		content, _ := os.ReadFile(filepath.Join(outDir, e.Name()))
		t.Logf("=== %s ===\n%s", e.Name(), content)
	}
}
