package validator

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.alis.build/alog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type (
	Validator struct {
		msgType  string
		protoMsg proto.Message

		rules      []*Rule
		fieldIndex map[string]int

		StringGetter     func(msg protoreflect.ProtoMessage, path string) (string, error)
		IntGetter        func(msg protoreflect.ProtoMessage, path string) (int64, error)
		FloatGetter      func(msg protoreflect.ProtoMessage, path string) (float64, error)
		BoolGetter       func(msg protoreflect.ProtoMessage, path string) (bool, error)
		EnumGetter       func(msg protoreflect.ProtoMessage, path string) (protoreflect.EnumNumber, error)
		SubMessageGetter func(msg protoreflect.ProtoMessage, path string) (protoreflect.ProtoMessage, error)

		StringListGetter     func(msg protoreflect.ProtoMessage, path string) ([]string, error)
		IntListGetter        func(msg protoreflect.ProtoMessage, path string) ([]int64, error)
		FloatListGetter      func(msg protoreflect.ProtoMessage, path string) ([]float64, error)
		BoolListGetter       func(msg protoreflect.ProtoMessage, path string) ([]bool, error)
		EnumListGetter       func(msg protoreflect.ProtoMessage, path string) ([]protoreflect.EnumNumber, error)
		SubMessageListGetter func(msg protoreflect.ProtoMessage, path string) ([]protoreflect.ProtoMessage, error)

		options *ValidatorOptions
	}

	ValidatorOptions struct {
		IgnoreWarnings bool
	}

	// Either PathsToValidate or OnlyValidateFieldsSpecifiedIn can be set, but not both. None are required.
	SubMsgOptions struct {
		PathsToValidate               []string
		OnlyValidateFieldsSpecifiedIn string
		IsRepeated                    bool
	}
)

func NewValidator(protoMsg proto.Message, options *ValidatorOptions) *Validator {
	msgType := getMsgType(protoMsg)
	if protoMsg == nil {
		alog.Fatalf(context.Background(), "protoMsg is nil")
	}
	if options == nil {
		options = &ValidatorOptions{}
	}
	v := &Validator{
		msgType:    msgType,
		fieldIndex: make(map[string]int),
		protoMsg:   protoMsg,
		options:    options,
	}
	validators[msgType] = v
	return v
}

func (v *Validator) AddRule(rule *Rule) *Rule {
	if rule.v != nil {
		if rule.v != v {
			alog.Fatalf(context.Background(), "cannot use the same rule/condition with multiple validators")
		}
	}

	// set validator in rule so it and all of its arguments have a reference to the validator
	rule.setValidator(v)

	// add rule
	v.rules = append(v.rules, rule)
	return rule
}

func (v *Validator) AddSubMessageValidator(path string, subMsgValidator *Validator, options *SubMsgOptions) *Rule {
	if options == nil {
		options = &SubMsgOptions{}
	} else {
		if options.PathsToValidate != nil && options.OnlyValidateFieldsSpecifiedIn != "" {
			alog.Fatalf(context.Background(), "Either PathsToValidate or OnlyValidateFieldsSpecifiedIn can be set, but not both")
		}
		if options.OnlyValidateFieldsSpecifiedIn != "" {
			v.getStringList(v.protoMsg, options.OnlyValidateFieldsSpecifiedIn)
		}
	}

	// validate path and ensure the type of the msg returned is the same as the subMsgValidator
	if options.IsRepeated {
		_ = v.getSubMessageList(v.protoMsg, path)
	} else {
		subM := v.getSubMessage(v.protoMsg, path)
		if getMsgType(subM) != subMsgValidator.msgType {
			alog.Fatalf(context.Background(), "subMsgValidator must be for the same type as the sub message at path %s", path)
		}

	}

	description := fmt.Sprintf("%s must be a valid %s", path, subMsgValidator.msgType)
	notDescription := fmt.Sprintf("%s must not be a valid %s", path, subMsgValidator.msgType)
	condDescr := fmt.Sprintf("%s is a valid %s", path, subMsgValidator.msgType)
	condNotDescr := fmt.Sprintf("%s is not a valid %s", path, subMsgValidator.msgType)
	if options.IsRepeated {
		description = fmt.Sprintf("each %s in %s must be valid", subMsgValidator.msgType, path)
		notDescription = fmt.Sprintf("each %s in %s must not be valid", subMsgValidator.msgType, path)
		condDescr = fmt.Sprintf("each %s in %s is valid", subMsgValidator.msgType, path)
		condNotDescr = fmt.Sprintf("each %s in %s is not valid", subMsgValidator.msgType, path)
	}

	// create open rule
	openRule := &pbOpen.Rule{
		Id:          path,
		Description: description,
		FieldPaths:  []string{path},
	}

	// add condition to description if options are set
	if options.OnlyValidateFieldsSpecifiedIn != "" {
		openRule.Description += fmt.Sprintf(", but only fields specified in %s are validated", options.OnlyValidateFieldsSpecifiedIn)
	} else if options.PathsToValidate != nil {
		openRule.Description += fmt.Sprintf(", but only %s are validated", strings.Join(options.PathsToValidate, ", "))
	}

	// only set nested rules if subMsgValidator is not for the same type as the parent validator
	if subMsgValidator.msgType != v.msgType {
		openRule.NestedRules = subMsgValidator.GetRules()
	}

	// create rule
	rule := &Rule{
		openRule:         openRule,
		notDescription:   notDescription,
		conditionDesc:    condDescr,
		conditionNotDesc: condNotDescr,
	}

	// set getViolations function
	if options.IsRepeated {
		rule.getViolations = func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
			subMsgs := v.getSubMessageList(msg, path)
			// get paths to validate if not all
			pathsToValidate := []string{}
			if options.OnlyValidateFieldsSpecifiedIn != "" {
				pathsToValidate = rule.v.getStringList(msg, options.OnlyValidateFieldsSpecifiedIn)
			} else if options.PathsToValidate != nil {
				pathsToValidate = options.PathsToValidate
			}
			allViols := []*pbOpen.Rule{}
			for i, subM := range subMsgs {
				viols, err := subMsgValidator.GetViolations(subM, pathsToValidate)
				if err != nil {
					return nil, err
				}
				for _, viol := range viols {
					fieldPaths := []string{}
					for _, fieldPath := range viol.FieldPaths {
						fieldPaths = append(fieldPaths, path+"."+fieldPath)
					}
					allViols = append(allViols, &pbOpen.Rule{
						Id:          viol.Id,
						Description: fmt.Sprintf("invalid %s at index %d: %s", path, i, viol.Description),
						FieldPaths:  fieldPaths,
					})
				}
			}
			return allViols, nil
		}
	} else {
		rule.getViolations = func(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
			// get sub message
			subM := rule.v.getSubMessage(msg, path)
			if subM == nil {
				return []*pbOpen.Rule{rule.openRule}, nil
			}

			// get paths to validate if not all
			pathsToValidate := []string{}
			if options.OnlyValidateFieldsSpecifiedIn != "" {
				pathsToValidate = rule.v.getStringList(msg, options.OnlyValidateFieldsSpecifiedIn)
			} else if options.PathsToValidate != nil {
				pathsToValidate = options.PathsToValidate
			}

			// get violations
			viols, err := subMsgValidator.GetViolations(subM, pathsToValidate)
			if err != nil {
				return nil, err
			}

			// return violations with path added as prefix to all fields
			allViols := []*pbOpen.Rule{}
			for _, viol := range viols {
				fieldPaths := []string{}
				for _, fieldPath := range viol.FieldPaths {
					fieldPaths = append(fieldPaths, path+"."+fieldPath)
				}
				allViols = append(allViols, &pbOpen.Rule{
					Id:          viol.Id,
					Description: fmt.Sprintf("invalid %s: %s", path, viol.Description),
					FieldPaths:  fieldPaths,
				})
			}
			return allViols, nil
		}
	}

	// add rule
	rule.setValidator(v)
	v.rules = append(v.rules, rule)
	return rule
}

func (v *Validator) GetViolations(msg protoreflect.ProtoMessage, fieldPaths []string) ([]*pbOpen.Rule, error) {
	if msg == nil {
		return nil, status.Errorf(codes.InvalidArgument, "message cannot be nil")
	}
	allViolations := []*pbOpen.Rule{}
	for _, rule := range v.rules {

		// if only specific fieldPaths are provided, skip rules with fieldPaths that are not in the list
		if len(fieldPaths) > 0 {
			skip := false
			for _, ruleFieldPath := range rule.openRule.FieldPaths {
				if !slices.Contains(fieldPaths, ruleFieldPath) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		// check if the rule should run based on potential conditions defined on the rule
		runRule, err := rule.shouldRun(msg)
		if err != nil {
			return nil, err
		}
		if !runRule {
			continue
		}

		// get violations for the rule
		viols, err := rule.getViolations(msg)
		if err != nil {
			return nil, err
		}
		allViolations = append(allViolations, viols...)
	}
	return allViolations, nil
}

func (v *Validator) GetRules() []*pbOpen.Rule {
	rules := make([]*pbOpen.Rule, len(v.rules))
	for i, r := range v.rules {
		rules[i] = r.openRule
	}
	return rules
}

func (v *Validator) Validate(msg protoreflect.ProtoMessage, fieldPaths []string) error {
	violations, err := v.GetViolations(msg, fieldPaths)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		errorStrings := make([]string, len(violations))
		for i, v := range violations {
			errorStrings[i] = v.Description
		}
		return status.Errorf(codes.InvalidArgument, "%s", strings.Join(errorStrings, "; "))
	}
	return nil
}

func (v *Validator) issueWarning(format string, a ...any) {
	if !v.options.IgnoreWarnings {
		alog.Warnf(context.Background(), fmt.Sprintf(format, a...))
	}
}
