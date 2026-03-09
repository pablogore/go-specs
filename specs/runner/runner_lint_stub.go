//go:build lint

package runner

import (
	"testing"

	specs "github.com/pablogore/go-specs/specs"
	"github.com/pablogore/go-specs/report"
)

func Run(_ *testing.T, _ string, _ *report.Reporter, _ func(s *specs.Spec)) {}

func RunWithOutput(_ *testing.T, _ string, _ func(s *specs.Spec)) {}
