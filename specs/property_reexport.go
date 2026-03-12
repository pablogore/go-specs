// property_reexport re-exports property-testing types and constructors from specs/property
// so the public API remains specs.PathValues, specs.PathGenerator, etc.
package specs

import "github.com/pablogore/go-specs/specs/property"

// Type aliases for property-testing types (implementations live in specs/property).
type (
	PathValues       = property.PathValues
	PathGenerator    = property.PathGenerator
	PathVar          = property.PathVar
	PathFilter       = property.PathFilter
	PathIterator     = property.PathIterator
	ValueShrinker    = property.ValueShrinker
	Coverage         = property.Coverage
	Corpus           = property.Corpus
	Mutator          = property.Mutator
	Shrinker         = property.Shrinker
	CoverageExplorer = property.CoverageExplorer
	SmartExplorer    = property.SmartExplorer
	ExplorationMode  = property.ExplorationMode
)

const (
	CartesianMode      = property.CartesianMode
	SamplingMode       = property.SamplingMode
	ExplorationGuided  = property.ExplorationGuided
)

var (
	NewPathGenerator            = property.NewPathGenerator
	NewPathGeneratorWithIntRange = property.NewPathGeneratorWithIntRange
	NewCorpus                   = property.NewCorpus
	NewMutator                  = property.NewMutator
	NewShrinker                 = property.NewShrinker
	NewCoverageExplorer         = property.NewCoverageExplorer
	NewSmartExplorer            = property.NewSmartExplorer
	DefaultIntShrinker          = property.DefaultIntShrinker
	DefaultBoolShrinker         = property.DefaultBoolShrinker
	DefaultFloatShrinker        = property.DefaultFloatShrinker
	IntRangeVar                 = property.IntRangeVar
)
