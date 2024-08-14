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
	"google.golang.org/protobuf/types/known/fieldmaskpb"
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

	// Set IgnoreWarnings to true to suppress warnings about missing getters
	// Provide ServiceName if the message type is a request message for more than one service that your server is implementing
	ValidatorOptions struct {
		// Set to true to suppress warnings about missing getters
		IgnoreWarnings bool
		// Format: pkg.ServiceName
		// E.g myorg.myapp.v1.MyService
		ServiceName string
	}

	// Both PathsToValidate and ValidateFieldsSpecifiedIn can be set, but none are required. Example of setting both is ValidateFieldPathsSpecifiedIn = "update_mask" and PathsToValidate = []string{"name"}
	SubMsgOptions struct {
		PathsToValidate               []string
		ValidateFieldPathsSpecifiedIn string
		IsRepeated                    bool
		MayBeEmpty                    bool
	}
)

// Create a new validator for a proto message. The type is inferred from the proto message.
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
	if validators[msgType] == nil {
		validators[msgType] = make(map[string]*Validator)
	}
	serviceName := options.ServiceName
	if serviceName == "" {
		serviceName = "*"
	}
	validators[msgType][serviceName] = v
	return v
}

// Add a top-level field rule to the validator. The rule will be run when Validate is called on the validator.
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

// Add another validator as a sub-message validator. The sub-message validator will be run when Validate is called on the parent validator.
func (v *Validator) AddSubMessageValidator(path string, subMsgValidator *Validator, options *SubMsgOptions) *Rule {
	fieldPathsGetter := func(v *Validator, msg protoreflect.ProtoMessage) []string {
		return []string{}
	}
	if options == nil {
		options = &SubMsgOptions{}
	} else {
		if options.ValidateFieldPathsSpecifiedIn != "" {
			fd, err := v.getFieldDescriptor(v.protoMsg, options.ValidateFieldPathsSpecifiedIn)
			if err != nil {
				alog.Fatalf(context.Background(), "field descriptor not found for %s", options.ValidateFieldPathsSpecifiedIn)
			}
			if fd.Kind() == protoreflect.StringKind && fd.Cardinality() == protoreflect.Repeated {
				fieldPathsGetter = func(v *Validator, msg protoreflect.ProtoMessage) []string {
					result := v.getStringList(msg, options.ValidateFieldPathsSpecifiedIn)
					return append(result, options.PathsToValidate...)
				}
			} else if fd.Kind() == protoreflect.MessageKind && fd.Cardinality() != protoreflect.Repeated {
				// ensure type of msg is google.protobuf.FieldMask
				if getMsgType(v.getSubMessage(v.protoMsg, options.ValidateFieldPathsSpecifiedIn)) != "google.protobuf.FieldMask" {
					alog.Fatalf(context.Background(), "ValidateFieldsSpecifiedIn must be a repeated string or field_mask")
				}
				fieldPathsGetter = func(v *Validator, msg protoreflect.ProtoMessage) []string {
					subM := v.getSubMessage(msg, options.ValidateFieldPathsSpecifiedIn)
					// cast to field_mask
					fieldMask := (subM).(*fieldmaskpb.FieldMask)
					result := fieldMask.GetPaths()
					return append(result, options.PathsToValidate...)
				}
			} else {
				alog.Fatalf(context.Background(), "ValidateFieldsSpecifiedIn must be a repeated string or field_mask")
			}
		} else {
			fieldPathsGetter = func(_ *Validator, _ protoreflect.ProtoMessage) []string {
				return options.PathsToValidate
			}
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
	if options.ValidateFieldPathsSpecifiedIn != "" {
		if len(options.PathsToValidate) > 0 {
			openRule.Description += fmt.Sprintf(", but only %s and fields specified in %s are validated", strings.Join(options.PathsToValidate, ","), options.ValidateFieldPathsSpecifiedIn)
		} else {
			openRule.Description += fmt.Sprintf(", but only fields specified in %s are validated", options.ValidateFieldPathsSpecifiedIn)
		}
	} else if len(options.PathsToValidate) > 0 {
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
			allViols := []*pbOpen.Rule{}
			for i, subM := range subMsgs {
				if subM == nil || !subM.ProtoReflect().IsValid() {
					if !options.MayBeEmpty {
						allViols = append(allViols, &pbOpen.Rule{
							Id:          path,
							Description: fmt.Sprintf("invalid %s at index %d: must not be empty", path, i),
							FieldPaths:  []string{path},
						})
					}
					continue
				}
				viols, err := subMsgValidator.GetViolations(subM, fieldPathsGetter(v, msg))
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
			if subM == nil || !subM.ProtoReflect().IsValid() {
				if !options.MayBeEmpty {
					return []*pbOpen.Rule{{
						Id:          path,
						Description: fmt.Sprintf("invalid %s: may not be empty", path),
						FieldPaths:  []string{path},
					}}, nil
				}
				return nil, nil
			}

			// get violations
			viols, err := subMsgValidator.GetViolations(subM, fieldPathsGetter(v, msg))
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

// Get violations for a proto message. If fieldPaths are provided, only rules with fieldPaths that are in the list will be run.
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
