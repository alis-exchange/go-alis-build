package validation_test

import (
	"testing"

	"go.alis.build/validation"
)

func TestCustomRuleMessageDefaultsEmpty(t *testing.T) {
	v := validation.NewValidator()
	if got := v.Custom("get.ok", false).Message(); got != "" {
		t.Fatalf("Message() = %q, want empty", got)
	}
}

func TestCustomRuleWithMessage(t *testing.T) {
	v := validation.NewValidator()
	rule := v.Custom("get.ok", false)
	if got := rule.WithMessage("NotFound: skill not found"); got != rule {
		t.Fatal("WithMessage returned a different rule")
	}
	if got := rule.Message(); got != "NotFound: skill not found" {
		t.Fatalf("Message() = %q, want the detail", got)
	}
	if got := rule.Rule(); got != "get.ok" {
		t.Fatalf("Rule() = %q, want the description", got)
	}
}

func TestCustomRuleMessageDiscoverableFromRule(t *testing.T) {
	v := validation.NewValidator()
	v.Custom("get.ok", false).WithMessage("detail")
	rules := v.Rules()
	if len(rules) != 1 {
		t.Fatalf("len(Rules()) = %d, want 1", len(rules))
	}
	m, ok := rules[0].(interface{ Message() string })
	if !ok || m.Message() != "detail" {
		t.Fatalf("rule does not expose the detail through interface{ Message() string }")
	}
}

func TestCustomRuleMessageLeavesRuleSemanticsUnchanged(t *testing.T) {
	plain := validation.NewValidator()
	plain.Custom("a.ok", true)
	plain.Custom("b.ok", false)

	detailed := validation.NewValidator()
	detailed.Custom("a.ok", true).WithMessage("ignored when passing")
	detailed.Custom("b.ok", false).WithMessage("b failed")

	if got, want := len(detailed.Rules()), len(plain.Rules()); got != want {
		t.Fatalf("len(Rules()) = %d, want %d", got, want)
	}
	broken := detailed.BrokenRules()
	if len(broken) != 1 || broken[0].Rule() != "b.ok" {
		t.Fatalf("BrokenRules() = %v, want only b.ok", broken)
	}
	if got, want := detailed.Validate().Error(), plain.Validate().Error(); got != want {
		t.Fatalf("Validate() = %q, want %q", got, want)
	}
}
