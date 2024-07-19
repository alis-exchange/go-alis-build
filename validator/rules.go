package validator

import (
	"context"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type Rule struct {
	Id             string
	Description    string
	NotDescription string
	isViolated     func(msg protoreflect.ProtoMessage) (bool, error)
	arguments      []argI
	condition      *Rule
	fieldPaths     []string
}

type argI interface {
	fieldPaths() []string
	setValidator(*Validator)
	getDescription() string
}

func NewRule(rule *Rule) *Rule {
	// if id, description, notDescription, isViolated or fieldPaths are empty, panic
	if rule.Id == "" {
		alog.Fatalf(context.Background(), "rule id is empty")
	}
	if rule.Description == "" {
		alog.Fatalf(context.Background(), "rule description is empty")
	}
	if rule.NotDescription == "" {
		alog.Fatalf(context.Background(), "rule not description is empty")
	}
	if rule.isViolated == nil {
		alog.Fatalf(context.Background(), "rule isViolated is nil")
	}

	if len(rule.fieldPaths) == 0 {
		fieldsSet := map[string]bool{}
		for _, arg := range rule.arguments {
			for _, fieldPath := range arg.fieldPaths() {
				if fieldsSet[fieldPath] {
					continue
				}
				rule.fieldPaths = append(rule.fieldPaths, fieldPath)
				fieldsSet[fieldPath] = true
			}
		}
	}
	if len(rule.fieldPaths) == 0 {
		alog.Fatalf(context.Background(), "rule fieldPaths is empty and no arguments have fieldPaths")
	}
	return rule
}

func runRules(v *Validator, msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
	allViolations := []*pbOpen.Rule{}
	for _, rule := range v.rules {
		runRule, err := rule.shouldRun(msg)
		if err != nil {
			return nil, err
		}
		if !runRule {
			continue
		}
		// startT := time.Now()
		ruleFailed, err := rule.isViolated(msg)
		if err != nil {
			return nil, err
		}
		// alog.Infof(context.Background(), "time: %v", time.Since(startT))
		if ruleFailed {
			allViolations = append(allViolations, rule.OpenRule())
		}
	}
	return allViolations, nil
}

func (r *Rule) ApplyIf(rule *Rule) *Rule {
	if rule.condition != nil {
		alog.Fatalf(context.Background(), "a condition to a rule cannot have a condition")
	}
	if r.condition != nil {
		alog.Fatalf(context.Background(), "rule already has a condition")
	}

	r.condition = rule
	return r
}

func (r *Rule) shouldRun(msg protoreflect.ProtoMessage) (bool, error) {
	if r.condition == nil {
		return true, nil
	}
	conditionFailed, err := r.condition.isViolated(msg)
	if err != nil {
		return false, err
	}
	return !conditionFailed, nil
}

func (r *Rule) OpenRule() *pbOpen.Rule {
	return &pbOpen.Rule{
		Id:          r.Id,
		Description: r.Description,
		FieldPaths:  r.fieldPaths,
	}
}

func NOT(rule *Rule) *Rule {
	if rule.condition != nil {
		alog.Fatalf(context.Background(), "cannot NOT a rule with a condition")
	}
	return &Rule{
		Description:    rule.NotDescription,
		NotDescription: rule.Description,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			ruleFailed, err := rule.isViolated(msg)
			if err != nil {
				return false, err
			}
			return !ruleFailed, nil
		},
		arguments:  rule.arguments,
		fieldPaths: rule.fieldPaths,
	}
}

func AND(rules ...*Rule) *Rule {
	ids := make([]string, len(rules))
	descriptions := make([]string, len(rules))
	notDescriptions := make([]string, len(rules))
	args := []argI{}
	for i, rule := range rules {
		if rule.condition != nil {
			alog.Fatalf(context.Background(), "cannot AND a rule with a condition")
		}
		ids[i] = rule.Id
		descriptions[i] = rule.Description
		notDescriptions[i] = rule.NotDescription
		args = append(args, rule.arguments...)
	}
	id := "A(" + strings.Join(ids, ",") + ")"
	descr := "(" + strings.Join(descriptions, " AND ") + ")"
	notDescription := "(" + strings.Join(notDescriptions, " OR ") + ")"

	return NewRule(&Rule{
		Id:             id,
		Description:    descr,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			for _, rule := range rules {
				ruleFailed, err := rule.isViolated(msg)
				if err != nil {
					return false, err
				}
				if ruleFailed {
					return true, nil
				}
			}
			return false, nil
		},
		arguments: args,
	})
}

func OR(rules ...*Rule) *Rule {
	ids := make([]string, len(rules))
	descriptions := make([]string, len(rules))
	notDescriptions := make([]string, len(rules))
	args := []argI{}
	for i, rule := range rules {
		if rule.condition != nil {
			alog.Fatalf(context.Background(), "cannot OR a rule with a condition")
		}
		ids[i] = rule.Id
		descriptions[i] = rule.Description
		notDescriptions[i] = rule.NotDescription
		args = append(args, rule.arguments...)
	}
	id := "O(" + strings.Join(ids, ",") + ")"
	descr := "(" + strings.Join(descriptions, " OR ") + ")"
	notDescription := "(" + strings.Join(notDescriptions, " AND ") + ")"

	return NewRule(&Rule{
		Id:             id,
		Description:    descr,
		NotDescription: notDescription,
		isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {
			for _, rule := range rules {
				ruleFailed, err := rule.isViolated(msg)
				if err != nil {
					return false, err
				}
				if !ruleFailed {
					return false, nil
				}
			}
			return true, nil
		},
		arguments: args,
	})
}

// func OR(conditions ...Condition) *Condition {
// 	descriptions := make([]string, len(conditions))
// 	notDescriptions := make([]string, len(conditions))
// 	for i, cond := range conditions {
// 		descriptions[i] = cond.Description
// 		notDescriptions[i] = cond.NotDescription
// 	}
// 	descr := "(" + strings.Join(descriptions, " OR ") + ")"
// 	notDescription := "(" + strings.Join(notDescriptions, " AND ") + ")"
// 	return &Condition{
// 		Description: descr,
// 		NotDescription: notDescription,
// 		getViolations: func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
// 			allViols := []*pbOpen.Violation{}
// 			for _, cond := range conditions {
// 				violations, err := cond.getViolations(data, fieldInfos)
// 				if err != nil {
// 					return violations, err
// 				}
// 				if len(violations) == 0 {
// 					return violations, nil
// 				}
// 				allViols = append(allViols, violations...)
// 			}
// 			return allViols, nil
// 		},
// 	}
// }

// func AND(conditions ...Condition) *Condition {
// 	descriptions := make([]string, len(conditions))
// 	notDescriptions := make([]string, len(conditions))
// 	for i, cond := range conditions {
// 		descriptions[i] = cond.Description
// 		notDescriptions[i] = cond.NotDescription
// 	}
// 	descr := "(" + strings.Join(descriptions, " AND ") + ")"
// 	notDescription := "(" + strings.Join(notDescriptions, " OR ") + ")"
// 	return &Condition{
// 		Description: descr,
// 		NotDescription: notDescription,
// 		getViolations: func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
// 			allViols := []*pbOpen.Violation{}
// 			for _, cond := range conditions {
// 				violations, err := cond.getViolations(data, fieldInfos)
// 				if err != nil {
// 					return violations, err
// 				}
// 				if len(violations) > 0 {
// 					return violations, nil
// 				}
// 				allViols = append(allViols, violations...)
// 			}
// 			return allViols, nil
// 		},
// 	}
// }

// // NewNotCondition creates a condition that the rule should only be run if some other condition is false
// func NOT(condition Condition) *Condition {
// 	return &Condition{
// 		Description: condition.NotDescription,
// 		NotDescription: condition.Description,
// 		getViolations: func(data interface{}, fieldInfos map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
// 			violations, err := condition.getViolations(data, fieldInfos)
// 			if err != nil {
// 				return violations, err
// 			}
// 			if len(violations) == 0 {
// 				for fieldPath,_ := range fieldInfos {
// 					violations = append(violations, &pbOpen.Violation{
// 						FieldPath: fieldPath,
// 						Message: fmt.Sprintf(),
// 					}
// 				}
// 			}
// 			return nil, nil

// 		}
// 	}
// }

// // ------------------------------- //

// func EnumIs(fieldPath string, val interface{}) *Condition {
// 	// enumValue := val.(protoreflect.Enum)
// 	enumValue, ok := val.(protoreflect.Enum)
// 	if !ok {
// 		alog.Fatalf(context.Background(), "val is not a protoreflect.Enum")
// 	}
// 	value := enumValue.Number()
// 	description := fmt.Sprintf("%s is equal to %s", fieldPath, enumValue.Descriptor().Values().Get(int(value)).Name())
// 	return &Condition{
// 		Description: description,
// 		shouldRunRule: func(data interface{}, conditionFieldInfos map[string]*FieldInfo) (bool, error) {
// 			return conditionFieldInfos[fieldPath].Value.Enum() == value, nil
// 		},
// 		allowedKinds:    []protoreflect.Kind{protoreflect.EnumKind},
// 		conditionFields: []string{fieldPath},
// 	}
// }

// func (v *Validator) IsPopulated(fieldPath string) *Condition {
// 	description := fmt.Sprintf("%s is populated", fieldPath)
// 	return &Condition{
// 		Description: description,
// 		shouldRunRule: func(data interface{}, conditionFieldInfos map[string]*FieldInfo) (bool, error) {
// 			fieldInfo := conditionFieldInfos[fieldPath]
// 			empty := fieldInfo.Value.Equal(fieldInfo.Descriptor.Default())
// 			return !empty, nil
// 		},
// 	}
// }
