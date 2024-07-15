package validator

import (
	"context"
	"fmt"

	"go.alis.build/alog"
	"go.alis.build/authz"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type Validator struct {
	msgType               string
	protoMsg              proto.Message
	az                    *authz.Authz
	rpcMethod             string
	iamResourceFieldPath  string
	validations           []func(data interface{}) ([]*pbOpen.Violation, error)
	rules                 []*pbOpen.Rule
	authorizedValidations []func(data interface{}, authInfo *authz.AuthInfo) ([]*pbOpen.Violation, error)
}

var validators = make(map[string]*Validator)

func NewValidator(msgType string, protoMsg proto.Message) *Validator {
	if msgType == "" {
		alog.Fatalf(context.Background(), "msgType is empty")
	}
	if protoMsg == nil {
		alog.Fatalf(context.Background(), "protoMsg is nil")
	}
	v := &Validator{
		msgType: msgType,
	}
	validators[msgType] = v
	return v
}

func NewValidatorWithAuthz(msgType string, protoMsg proto.Message, rpcMethod string, az *authz.Authz, iamResourceFieldPath string) *Validator {
	if rpcMethod == "" {
		alog.Fatalf(context.Background(), "rpcMethod is empty for %s", msgType)
	}
	if az == nil {
		alog.Fatalf(context.Background(), "az is nil")
	}
	if iamResourceFieldPath == "" {
		alog.Fatalf(context.Background(), "iamResourceFieldPath is empty for %s", msgType)
	}
	_, err := getStringField(protoMsg, iamResourceFieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "iam resource field path (%s) not found for %s", iamResourceFieldPath, msgType)
	}
	v := NewValidator(msgType, protoMsg)
	v.az = az
	v.iamResourceFieldPath = iamResourceFieldPath
	return v
}

func (v *Validator) AddRule(rule *pbOpen.Rule, validationFunction func(data interface{}) ([]*pbOpen.Violation, error)) {
	v.rules = append(v.rules, rule)
	v.validations = append(v.validations, validationFunction)
}

func (v *Validator) AddAuthorizedRule(rule *pbOpen.Rule, validationFunction func(data interface{}, authInfo *authz.AuthInfo) ([]*pbOpen.Violation, error), az *authz.Authz) {
	v.rules = append(v.rules, rule)
	v.authorizedValidations = append(v.authorizedValidations, validationFunction)
}

func (v *Validator) HandleValidateRpc(req *pbOpen.ValidateMessageRequest) (*pbOpen.ValidateMessageResponse, error) {
	if v.msgType != req.MsgType {
		return nil, status.Errorf(codes.Internal, "msgType mismatch")
	}
	// clone proto message
	msg := proto.Clone(v.protoMsg)
	anypb.UnmarshalTo(req.Msg, msg, proto.UnmarshalOptions{})

	violations, err := v.GetViolations(msg, req.ShowAuthorizedViolations)
	if err != nil {
		return nil, err
	}

	resp := &pbOpen.ValidateMessageResponse{
		Violations: violations,
	}
	return resp, nil
}

func HandleValidateRpc(req *pbOpen.ValidateMessageRequest) (*pbOpen.ValidateMessageResponse, error) {
	v, ok := validators[req.MsgType]
	if !ok {
		return nil, status.Errorf(codes.Internal, "msgType not found")
	}
	return v.HandleValidateRpc(req)
}

func (v *Validator) RetrieveRulesRpc(req *pbOpen.RetrieveRulesRequest) (*pbOpen.RetrieveRulesResponse, error) {
	if v.msgType != req.MsgType {
		return nil, status.Errorf(codes.Internal, "msgType mismatch")
	}

	resp := &pbOpen.RetrieveRulesResponse{
		Rules: v.rules,
	}
	return resp, nil
}

func RetrieveRulesRpc(req *pbOpen.RetrieveRulesRequest) (*pbOpen.RetrieveRulesResponse, error) {
	v, ok := validators[req.MsgType]
	if !ok {
		return nil, status.Errorf(codes.Internal, "msgType not found")
	}
	return v.RetrieveRulesRpc(req)
}

func (v *Validator) GetViolations(msg interface{}, includeAuthorizedValidations bool) ([]*pbOpen.Violation, error) {
	var allViolations []*pbOpen.Violation
	for _, f := range v.validations {
		violations, err := f(msg)
		if err != nil {
			return nil, err
		}
		allViolations = append(allViolations, violations...)
	}

	if v.az != nil && includeAuthorizedValidations {
		data := msg.(*proto.Message)
		// extract iam resource from the message and ensure the field is a string
		iamResource, err := getStringField(*data, v.iamResourceFieldPath)
		if err != nil {
			return nil, err
		}

		authInfo, err := v.az.AuthorizeFromResources(context.Background(), v.rpcMethod, []string{iamResource}, nil)
		if err != nil {
			return nil, err
		}
		for _, f := range v.authorizedValidations {
			violations, err := f(msg, authInfo)
			if err != nil {
				return nil, err
			}
			allViolations = append(allViolations, violations...)
		}
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
		for _, v := range violations {
			errorStrings = append(errorStrings, fmt.Sprintf("%s:%s (rule %s)", v.FieldPath, v.Message, v.RuleId))
		}

		return status.Errorf(codes.InvalidArgument, "%s", errorStrings)
	}
	return nil
}

func Validate(msg interface{}) (error, bool) {
	protoMsg := msg.(*proto.Message)
	msgType := getMsgType(*protoMsg)
	v, ok := validators[msgType]
	if !ok {
		return nil, false
	}
	return v.Validate(msg), true
}
