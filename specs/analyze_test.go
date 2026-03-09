package specs

import "testing"

func TestAnalyzeBuildsSpecTree(t *testing.T) {
	suite := Analyze(func() {
		Describe(nil, "Root", func(s *Spec) {
			s.It("leaf", func(ctx *Context) {})
		})
	})
	if suite == nil || suite.Arena == nil {
		t.Fatalf("expected suite root")
	}
	rootID := suite.RootID
	if len(suite.Arena.Children[rootID]) != 1 {
		t.Fatalf("expected one root child, got %d", len(suite.Arena.Children[rootID]))
	}
	descID := suite.Arena.Children[rootID][0]
	desc := &suite.Arena.Nodes[descID]
	if desc.Name != "Root" || desc.Type != DescribeNode {
		t.Fatalf("expected describe node, got %+v", desc)
	}
	if len(suite.Arena.Children[descID]) != 1 {
		t.Fatalf("expected one leaf under describe, got %d", len(suite.Arena.Children[descID]))
	}
	leID := suite.Arena.Children[descID][0]
	le := &suite.Arena.Nodes[leID]
	if le.Name != "leaf" || le.Type != ItNode {
		t.Fatalf("expected leaf it node, got %+v", le)
	}
}
