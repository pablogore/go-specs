package main

import (
	"os"

	"github.com/pablogore/go-specs/internal/cli"
	"github.com/pablogore/go-specs/internal/cli/bench"
	"github.com/pablogore/go-specs/internal/cli/coverage"
	"github.com/pablogore/go-specs/internal/cli/doctor"
	"github.com/pablogore/go-specs/internal/cli/generate"
	"github.com/pablogore/go-specs/internal/cli/migrate"
	"github.com/pablogore/go-specs/internal/cli/testcmd"
)

func main() {
	root := cli.NewRoot()

	coverage.Register(root)
	testcmd.Register(root)
	bench.Register(root)
	doctor.Register(root)
	generate.Register(root)
	migrate.Register(root)

	// Top-level "run" = test run (backward compatibility)
	root.RegisterTopLevel("run", testcmd.RunTestRun)

	os.Exit(root.Execute())
}
