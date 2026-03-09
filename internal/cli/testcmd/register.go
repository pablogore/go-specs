package testcmd

import (
	"github.com/pablogore/go-specs/internal/cli"
)

// Register adds test commands to the root CLI.
func Register(root *cli.Root) {
	root.RegisterGroup("test", map[string]cli.Runner{
		"run":     RunTestRun,
		"tree":    RunTree,
		"stats":   RunStats,
		"generate": RunGenerate,
		"migrate": RunMigrate,
	})
}
