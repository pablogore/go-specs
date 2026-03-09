//go:build lint

package specs

func (s *Spec) Adversarial(_ string, _ []string, _ func(input string)) {}

func (s *Spec) AdversarialWithContext(_ string, _ []string, _ func(ctx *Context, input string)) {}

func AdversarialInputs() []string       { return nil }
func EmptyStringGenerator() []string    { return nil }
func WhitespaceGenerator() []string     { return nil }
func InvalidUTF8Generator() []string    { return nil }
func VeryLongStringGenerator() []string { return nil }
