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
	"google.golang.org/protobuf/types/known/anypb"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

type (
	Func           func(data interface{}, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error)
	AuthorizedFunc func(data interface{}, authInfo *authz.AuthInfo, alreadyViolatedFields map[string]bool) ([]*pbOpen.Violation, error)
	Validator      struct {
		msgType               string
		protoMsg              proto.Message
		az                    *authz.Authz
		rpcMethod             string
		iamResourceFieldPath  string
		validations           []Func
		rules                 []*pbOpen.Rule
		authorizedValidations []AuthorizedFunc
		FieldIndex            map[string]int
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
	_, err := v.GetStringField(protoMsg, iamResourceFieldPath)
	if err != nil {
		alog.Fatalf(context.Background(), "iam resource field path (%s) not found for %s", iamResourceFieldPath, v.msgType)
	}
	v.az = az
	v.iamResourceFieldPath = iamResourceFieldPath
	return v
}

func (v *Validator) AddRule(ruleDescription string, fieldPaths []string, validationFunction Func) {
	rule := &pbOpen.Rule{
		Id:          fmt.Sprintf("%d", len(v.rules)+1),
		Description: ruleDescription,
		FieldPaths:  fieldPaths,
	}
	v.rules = append(v.rules, rule)
	v.validations = append(v.validations, validationFunction)
}

func (v *Validator) AddAuthorizedRule(ruleDescription string, fieldPaths []string, validationFunction AuthorizedFunc, az *authz.Authz) {
	if az == nil {
		alog.Fatalf(context.Background(), "cannot add authorized rule without authz")
	}
	rule := &pbOpen.Rule{
		Id:          fmt.Sprintf("%d", len(v.rules)+1),
		Description: ruleDescription,
		FieldPaths:  fieldPaths,
	}
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
	alreadyViolatedFields := map[string]bool{}
	for i, f := range v.validations {
		violations, err := f(msg, alreadyViolatedFields)
		if err != nil {
			return nil, err
		}
		for _, val := range violations {
			val.RuleId = v.rules[i].Id
			alreadyViolatedFields[val.FieldPath] = true
			allViolations = append(allViolations, val)
		}
	}
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
		for i, f := range v.authorizedValidations {
			violations, err := f(msg, authInfo, alreadyViolatedFields)
			if err != nil {
				return nil, err
			}
			for _, val := range violations {
				val.RuleId = v.rules[i].Id
				allViolations = append(allViolations, val)
			}
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
		for i, v := range violations {
			errorStrings[i] = fmt.Sprintf("%s (rule: %s,field: %s)", v.Message, v.RuleId, v.FieldPath)
		}

		return status.Errorf(codes.InvalidArgument, "%s", strings.Join(errorStrings, "; "))
	}
	return nil
}

func Validate(msg interface{}) (error, bool) {
	protoMsg := msg.(protoreflect.ProtoMessage)
	msgType := GetMsgType(protoMsg)
	v, ok := validators[msgType]
	if !ok {
		return nil, false
	}
	return v.Validate(msg), true
}

func (v *Validator) GetStringField(data interface{}, fieldName string) (string, error) {
	val, err := v.GetFieldValue(data, fieldName, []protoreflect.Kind{protoreflect.StringKind})
	if err != nil {
		return "", err
	}

	return val.String(), nil
}

func (v *Validator) GetIntField(data interface{}, fieldName string) (int, error) {
	val, err := v.GetFieldValue(data, fieldName, []protoreflect.Kind{protoreflect.Int32Kind, protoreflect.Int64Kind})
	if err != nil {
		return 0, err
	}

	return int(val.Int()), nil
}

func (v *Validator) GetFloatField(data interface{}, fieldName string) (float64, error) {
	val, err := v.GetFieldValue(data, fieldName, []protoreflect.Kind{protoreflect.FloatKind, protoreflect.DoubleKind})
	if err != nil {
		return 0, err
	}
	return val.Float(), nil
}

func (v *Validator) GetFieldValue(data interface{}, fieldName string, allowedTypes []protoreflect.Kind) (*protoreflect.Value, error) {
	msg := data.(protoreflect.ProtoMessage)
	parentMsg, fieldName, err := GetFieldParentMessage(msg, fieldName)
	if err != nil {
		return nil, err
	}
	md := parentMsg.ProtoReflect().Descriptor()
	index := 0
	if val, ok := v.FieldIndex[fieldName]; ok {
		index = val
	} else {
		foundIndex := false
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			if fd.TextName() == fieldName {
				v.FieldIndex[fieldName] = i
				index = i
				foundIndex = true
			}
		}
		if !foundIndex {
			return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
		}
	}

	// validate
	fd := md.Fields().Get(index)
	if fd.TextName() != fieldName {
		return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
	}
	foundType := false
	if allowedTypes == nil {
		foundType = true
	} else {
		for _, allowedType := range allowedTypes {
			if fd.Kind() == allowedType {
				foundType = true
				break
			}
		}
	}
	if !foundType {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not a valid field type", fieldName)
	}
	value := parentMsg.ProtoReflect().Get(fd)
	return &value, nil
}

func (v *Validator) GetFieldDescriptor(data interface{}, fieldName string) (protoreflect.FieldDescriptor, error) {
	msg := data.(protoreflect.ProtoMessage)
	parentMsg, fieldName, err := GetFieldParentMessage(msg, fieldName)
	if err != nil {
		return nil, err
	}
	md := parentMsg.ProtoReflect().Descriptor()
	index := 0
	if val, ok := v.FieldIndex[fieldName]; ok {
		index = val
	} else {
		foundIndex := false
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			if fd.TextName() == fieldName {
				v.FieldIndex[fieldName] = i
				index = i
				foundIndex = true
			}
		}
		if !foundIndex {
			return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
		}
	}

	// validate
	fd := md.Fields().Get(index)
	if fd.TextName() != fieldName {
		return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
	}
	return fd, nil
}
