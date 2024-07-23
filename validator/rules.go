package validator

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type Rule struct {
	openRule         *pbOpen.Rule
	notDescription   string
	conditionDesc    string
	conditionNotDesc string
	getViolations    func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error)
	arguments        []argI
	condition        *Rule
	v                *Validator
}

type Descriptions struct {
	rule         string
	notRule      string
	condition    string
	notCondition string
}

type argI interface {
	fieldPaths() []string
	getDescription() string
	getValidator() *Validator
	setValidator(*Validator)
}

func newPrimitiveRule(id string, descriptions *Descriptions, args []argI, isViolated func(msg protoreflect.ProtoMessage) (bool, error)) *Rule {
	// determine fieldPaths from arguments
	fieldPaths := []string{}
	fieldsSet := map[string]bool{}
	for _, arg := range args {
		for _, fieldPath := range arg.fieldPaths() {
			if fieldsSet[fieldPath] {
				continue
			}
			// do not allow nested fields
			if strings.Contains(fieldPath, ".") {
				alog.Fatalf(context.Background(), "nested field (%s) not allowed. Create a validator for the submessage and add it to the parent validator with parentVal.AddSubMessageValidator(...)", fieldPath)
			}
			fieldPaths = append(fieldPaths, fieldPath)
			fieldsSet[fieldPath] = true
		}
	}
	if len(fieldPaths) == 0 {
		alog.Fatalf(context.Background(), "rule with description=\"%s\" only has fixed values as arguments", descriptions.rule)
	}

	// define open rule
	openRule := &pbOpen.Rule{
		Id:          id,
		Description: descriptions.rule,
		FieldPaths:  fieldPaths,
	}

	// create rule
	rule := &Rule{
		openRule:         openRule,
		notDescription:   descriptions.notRule,
		conditionDesc:    descriptions.condition,
		conditionNotDesc: descriptions.notCondition,
		arguments:        args,
	}

	// set getViolations function
	rule.getViolations = func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
		ruleFailed, err := isViolated(msg)
		if err != nil {
			return nil, err
		}
		if ruleFailed {
			return []*pbOpen.Rule{openRule}, nil
		}
		return nil, nil
	}

	// return rule
	return rule
}

// Only apply the rule if the condition is met
func (r *Rule) ApplyIf(condition *Rule) *Rule {
	if condition.condition != nil {
		alog.Fatalf(context.Background(), "a condition to a rule cannot have a condition")
	}
	if r.condition != nil {
		alog.Fatalf(context.Background(), "rule already has a condition")
	}
	if condition == nil {
		alog.Fatalf(context.Background(), "condition cannot be nil")
	}
	if condition.v != nil && r.v != nil {
		if condition.v != r.v {
			alog.Fatalf(context.Background(), "condition to a rule must have the same validator")
		}
	}

	// set validator in condition just in case its already set, i.e. the rule was added to a validator and afterwards the ApplyIf was called on the rule
	if r.v != nil {
		condition.setValidator(r.v)
	}

	// change description and fields of rule to include condition
	r.openRule.Description = fmt.Sprintf("%s if %s", r.openRule.Description, condition.conditionDesc)
	r.openRule.FieldPaths = append(r.openRule.FieldPaths, condition.openRule.FieldPaths...)

	// set condition
	r.condition = condition
	return r
}

func (r *Rule) setValidator(v *Validator) {
	if r.v != nil {
		if r.v != v {
			alog.Fatalf(context.Background(), "rule with description=\"%s\" already has a different validator", r.openRule.Description)
		}
	}
	r.v = v
	for _, arg := range r.arguments {
		if arg.getValidator() != nil {
			if arg.getValidator() != v {
				alog.Fatalf(context.Background(), "arg with description=\"%s\" already has a different validator", arg.getDescription())
			}
		}
		arg.setValidator(v)
	}
	if r.condition != nil {
		if r.condition.v != nil {
			if r.condition.v != v {
				alog.Fatalf(context.Background(), "condition with description=\"%s\" already has a different validator", r.condition.openRule.Description)
			}
		}
		r.condition.setValidator(v)
	}
}

func (r *Rule) shouldRun(msg protoreflect.ProtoMessage) (bool, error) {
	if r.condition == nil {
		return true, nil
	}
	viols, err := r.condition.getViolations(msg)
	if err != nil {
		return false, err
	}
	return len(viols) == 0, nil
}

// NOT creates a new rule that is violated if the given rule is not violated
func NOT(rule *Rule) *Rule {
	if rule.condition != nil {
		alog.Fatalf(context.Background(), "cannot NOT a rule with a condition")
	}
	if rule.notDescription == "" {
		alog.Fatalf(context.Background(), "cannot NOT a rule without a notDescription")
	}

	// create new rule that reverses descriptions
	notRule := &Rule{
		openRule: &pbOpen.Rule{
			Id:          fmt.Sprintf("not(%s)", rule.openRule.Id),
			Description: rule.notDescription,
			FieldPaths:  rule.openRule.FieldPaths,
		},
		notDescription:   rule.openRule.Description,
		conditionDesc:    rule.conditionNotDesc,
		conditionNotDesc: rule.conditionDesc,
		arguments:        rule.arguments,
	}

	// set getViolations function to reverse the result
	notRule.getViolations = func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
		viols, err := rule.getViolations(msg)
		if err != nil {
			return nil, err
		}
		if len(viols) > 0 {
			return nil, nil
		}
		return []*pbOpen.Rule{rule.openRule}, nil
	}

	// return new rule
	return notRule
}

// AND creates a new rule that is violated if any of the rules are violated
func AND(rules ...*Rule) *Rule {
	// extract data from rules to use in new rule
	ids := make([]string, len(rules))
	descriptions := make([]string, len(rules))
	notDescriptions := make([]string, len(rules))
	condDescriptions := make([]string, len(rules))
	condNotDescriptions := make([]string, len(rules))
	args := []argI{}
	fieldPaths := map[string]bool{}
	for i, rule := range rules {
		if rule.condition != nil {
			alog.Fatalf(context.Background(), "cannot AND a rule with a condition")
		}
		if rule.notDescription == "" {
			alog.Fatalf(context.Background(), "cannot AND a rule without a notDescription")
		}
		if rule.conditionNotDesc == "" {
			alog.Fatalf(context.Background(), "cannot AND a rule without a conditionNotDesc")
		}
		if rule.conditionDesc == "" {
			alog.Fatalf(context.Background(), "cannot AND a rule without a conditionDesc")
		}
		ids[i] = rule.openRule.Id
		descriptions[i] = rule.openRule.Description
		notDescriptions[i] = rule.notDescription
		condDescriptions[i] = rule.conditionDesc
		condNotDescriptions[i] = rule.conditionNotDesc
		args = append(args, rule.arguments...)
		for _, fieldPath := range rule.openRule.FieldPaths {
			fieldPaths[fieldPath] = true
		}
	}

	// create open rule
	openRule := &pbOpen.Rule{
		Id:          "and(" + strings.Join(ids, ",") + ")",
		Description: "(" + strings.Join(descriptions, " AND ") + ")",
		FieldPaths:  maps.Keys(fieldPaths),
	}

	// create rule
	rule := &Rule{
		openRule:         openRule,
		notDescription:   "(" + strings.Join(notDescriptions, " OR ") + ")",
		conditionDesc:    "(" + strings.Join(condDescriptions, " AND ") + ")",
		conditionNotDesc: "(" + strings.Join(condNotDescriptions, " OR ") + ")",
		arguments:        args,
	}

	// set getViolations function
	rule.getViolations = func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
		for _, rule := range rules {
			ruleFailed, err := rule.getViolations(msg)
			if err != nil {
				return nil, err
			}
			if len(ruleFailed) > 0 {
				return []*pbOpen.Rule{openRule}, nil
			}
		}
		return nil, nil
	}

	// return rule
	return rule
}

// OR creates a new rule that is only violated if all of the rules are violated
func OR(rules ...*Rule) *Rule {
	// extract data from rules to use in new rule
	ids := make([]string, len(rules))
	descriptions := make([]string, len(rules))
	notDescriptions := make([]string, len(rules))
	condDescriptions := make([]string, len(rules))
	condNotDescriptions := make([]string, len(rules))
	args := []argI{}
	fieldPaths := map[string]bool{}
	for i, rule := range rules {
		if rule.condition != nil {
			alog.Fatalf(context.Background(), "cannot OR a rule with a condition")
		}
		if rule.notDescription == "" {
			alog.Fatalf(context.Background(), "cannot OR a rule without a notDescription")
		}
		if rule.conditionNotDesc == "" {
			alog.Fatalf(context.Background(), "cannot OR a rule without a conditionNotDesc")
		}
		if rule.conditionDesc == "" {
			alog.Fatalf(context.Background(), "cannot OR a rule without a conditionDesc")
		}
		ids[i] = rule.openRule.Id
		descriptions[i] = rule.openRule.Description
		notDescriptions[i] = rule.notDescription
		condDescriptions[i] = rule.conditionDesc
		condNotDescriptions[i] = rule.conditionNotDesc
		args = append(args, rule.arguments...)
		for _, fieldPath := range rule.openRule.FieldPaths {
			fieldPaths[fieldPath] = true
		}
	}

	// create open rule
	openRule := &pbOpen.Rule{
		Id:          "or(" + strings.Join(ids, ",") + ")",
		Description: "(" + strings.Join(descriptions, " OR ") + ")",
		FieldPaths:  maps.Keys(fieldPaths),
	}

	// create rule
	rule := &Rule{
		openRule:         openRule,
		notDescription:   "(" + strings.Join(notDescriptions, " AND ") + ")",
		conditionDesc:    "(" + strings.Join(condDescriptions, " OR ") + ")",
		conditionNotDesc: "(" + strings.Join(condNotDescriptions, " AND ") + ")",
		arguments:        args,
	}

	// set getViolations function
	rule.getViolations = func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
		for _, rule := range rules {
			ruleFailed, err := rule.getViolations(msg)
			if err != nil {
				return nil, err
			}
			if len(ruleFailed) == 0 {
				return nil, nil
			}
		}
		return []*pbOpen.Rule{openRule}, nil
	}

	// return rule
	return rule
}
