package validator

import (
	"context"
	"fmt"
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
		SubMessageGetter func(msg protoreflect.ProtoMessage, path string) (protoreflect.ProtoMessage, error)

		StringListGetter     func(msg protoreflect.ProtoMessage, path string) ([]string, error)
		IntListGetter        func(msg protoreflect.ProtoMessage, path string) ([]int64, error)
		FloatListGetter      func(msg protoreflect.ProtoMessage, path string) ([]float64, error)
		BoolListGetter       func(msg protoreflect.ProtoMessage, path string) ([]bool, error)
		SubMessageListGetter func(msg protoreflect.ProtoMessage, path string) ([]protoreflect.ProtoMessage, error)

		options *ValidatorOptions
	}

	ValidatorOptions struct {
		IgnoreWarnings bool
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

func (v *Validator) AddRule(rule *Rule) {
	for _, arg := range rule.arguments {
		arg.setValidator(v)
	}
	if rule.condition != nil {
		for _, arg := range rule.condition.arguments {
			arg.setValidator(v)
		}
	}
	v.rules = append(v.rules, rule)
}

func (v *Validator) GetViolations(msg protoreflect.ProtoMessage) ([]*pbOpen.Rule, error) {
	allViolations, err := runRules(v, msg)
	if err != nil {
		return nil, err
	}
	return allViolations, nil
}

func (v *Validator) GetRules() []*pbOpen.Rule {
	rules := make([]*pbOpen.Rule, len(v.rules))
	for i, r := range v.rules {
		rules[i] = r.OpenRule()
	}
	return rules
}

func (v *Validator) Validate(msg protoreflect.ProtoMessage) error {
	violations, err := v.GetViolations(msg)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		errorStrings := make([]string, len(violations))
		for i, v := range violations {
			errorStrings[i] = fmt.Sprintf("%s (fields: %s)", v.Description, strings.Join(v.FieldPaths, ","))
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
