package validator

import (
	"context"
	"fmt"
	"strings"

	"go.alis.build/alog"
	"go.alis.build/authz"
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

		validations           []*Validation
		authorizedValidations []*Validation
		FieldIndex            map[string]int

		az                   *authz.Authz
		rpcMethod            string
		iamResourceFieldPath string
	}
)

var validators = make(map[string]*Validator)

func NewValidator(protoMsg proto.Message) *Validator {
	msgType := GetMsgType(protoMsg)
	if protoMsg == nil {
		alog.Fatalf(context.Background(), "protoMsg is nil")
	}
	v := &Validator{
		msgType:    msgType,
		FieldIndex: make(map[string]int),
		protoMsg:   protoMsg,
	}
	validators[msgType] = v
	return v
}

func NewValidatorWithAuthz(protoMsg proto.Message, rpcMethod string, az *authz.Authz, iamResourceFieldPath string) *Validator {
	if rpcMethod == "" {
		alog.Fatalf(context.Background(), "rpcMethod is empty for %s", GetMsgType(protoMsg))
	}
	if az == nil {
		alog.Fatalf(context.Background(), "az is nil")
	}
	if iamResourceFieldPath == "" {
		alog.Fatalf(context.Background(), "iamResourceFieldPath is empty for %s", GetMsgType(protoMsg))
	}

	v := NewValidator(protoMsg)
	v.validateFieldPaths([]string{iamResourceFieldPath}, protoreflect.StringKind)
	v.az = az
	v.iamResourceFieldPath = iamResourceFieldPath
	return v
}

func (v *Validator) AddRule(ruleDescription string, fieldPaths []string, validationFunction Func) *Validation {
	v.validateFieldPaths(fieldPaths)
	return v.addRule("custom", ruleDescription, fieldPaths, validationFunction, nil, []string{})
}

func (v *Validator) AddAuthorizedRule(ruleDescription string, fieldPaths []string, validationFunction AuthorizedFunc) *Validation {
	v.validateFieldPaths(fieldPaths)
	return v.addRule("custom", ruleDescription, fieldPaths, nil, validationFunction, []string{})
}

func (v *Validator) GetViolations(msg interface{}, includeAuthorizedValidations bool) ([]*pbOpen.Violation, error) {
	msgType := GetMsgType(msg.(protoreflect.ProtoMessage))
	if msgType != v.msgType {
		return nil, status.Errorf(codes.Internal, "msgType mismatch")
	}
	alreadyViolatedFields := make(map[string]bool)
	fieldInfoCache := make(map[string]*FieldInfo)
	allViolations, err := v.runValidations(msg, alreadyViolatedFields, fieldInfoCache, nil)
	if err != nil {
		return nil, err
	}

	// do not proceed with authorized validations if there are already violations
	if len(allViolations) > 0 {
		return allViolations, nil
	}

	if v.az != nil && includeAuthorizedValidations {
		// extract iam resource from the message and ensure the field is a string
		iamResource, err := v.GetStringField(msg, v.iamResourceFieldPath)
		if err != nil {
			return nil, err
		}

		authInfo, err := v.az.AuthorizeFromResources(context.Background(), v.rpcMethod, []string{iamResource}, nil)
		if err != nil {
			return nil, err
		}

		authorizedViolations, err := v.runValidations(msg, alreadyViolatedFields, fieldInfoCache, authInfo)
		if err != nil {
			return nil, err
		}
		allViolations = append(allViolations, authorizedViolations...)
	}
	return allViolations, nil
}

func (v *Validator) Validate(msg interface{}) error {
	violations, err := v.GetViolations(msg, true)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		errorStrings := make([]string, len(violations))
		for i, v := range violations {
			errorStrings[i] = fmt.Sprintf("%s (rule: %s,field: %s)", v.Message, v.RuleId, v.FieldPath)
		}

		return status.Errorf(codes.InvalidArgument, "%s", strings.Join(errorStrings, "; "))
	}
	return nil
}

func Validate(msg interface{}) (error, bool) {
	v, found := locateValidator(msg)
	if !found {
		return status.Errorf(codes.Internal, "validator not found"), false
	}
	return v.Validate(msg), true
}
