package ctx

import "github.com/pablogore/go-specs/assert"

// Re-export core matchers so the public DSL remains specs.Equal, specs.BeTrue, etc.
func Equal(expected any) assert.Matcher   { return assert.Equal(expected) }
func NotEqual(expected any) assert.Matcher { return assert.NotEqual(expected) }
func BeNil() assert.Matcher                { return assert.BeNil() }
func BeTrue() assert.Matcher               { return assert.BeTrue() }
func BeFalse() assert.Matcher              { return assert.BeFalse() }
func Contain(expected any) assert.Matcher  { return assert.Contain(expected) }
