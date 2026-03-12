package property

// PathVar represents a named variable and its candidate values.
// Values and Shrinker are set by the DSL; rangeSpec is set by IntRangeVar.
type PathVar struct {
	Name      string
	Values    []any
	rangeSpec *intRange
	Shrinker  ValueShrinker
}

// IntRangeVar returns a PathVar for an integer range [min, max] (inclusive).
func IntRangeVar(name string, min, max int) PathVar {
	if name == "" || max < min {
		return PathVar{}
	}
	return PathVar{Name: name, rangeSpec: &intRange{min: min, max: max}}
}

// ValueShrinker defines how to reduce a failing value to simpler forms.
type ValueShrinker interface {
	Shrink(value any) []any
}

type intRange struct {
	min int
	max int
}

// PathFilter filters combinations during generation.
type PathFilter func(PathValues) bool
