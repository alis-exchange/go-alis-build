package validator

import (
	"context"
	"fmt"
	"slices"
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
	Options struct {
		ValidateEmptyFields           bool
		ValidateAlreadyViolatedFields bool
		ConditionFunc                 ConditionFunc
		AuthorizedConditionFunc       AuthorizedConditionFunc
		ConditionFields               []string
		ConditionDescription          string
	}
	Validation struct {
		Rule    *pbOpen.Rule
		Options *Options
		Func    Func
	}
	AuthorizedValidation struct {
		Rule    *pbOpen.Rule
		Options *Options
		Func    AuthorizedFunc
	}

	FieldInfo struct {
		Value      protoreflect.Value
		Descriptor protoreflect.FieldDescriptor
	}
	Func                    func(data interface{}, fields map[string]*FieldInfo) ([]*pbOpen.Violation, error)
	ConditionFunc           func(data interface{}, condition_fields map[string]*FieldInfo) (bool, error)
	AuthorizedFunc          func(data interface{}, fields map[string]*FieldInfo, authInfo *authz.AuthInfo) ([]*pbOpen.Violation, error)
	AuthorizedConditionFunc func(data interface{}, condition_fields map[string]*FieldInfo, authInfo *authz.AuthInfo) (bool, error)
	Validator               struct {
		msgType  string
		protoMsg proto.Message

		validations           []*Validation
		authorizedValidations []*AuthorizedValidation
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

func (v *Validator) AddRule(ruleDescription string, fieldPaths []string, validationFunction Func, options *Options) {
	v.validateFieldPaths(fieldPaths)
	v.addRule(ruleDescription, fieldPaths, validationFunction, options)
}

func (v *Validator) addRule(ruleDescription string, fieldPaths []string, validationFunction Func, options *Options) {
	rule := &pbOpen.Rule{
		Id:          fmt.Sprintf("%d", len(v.validations)+len(v.authorizedValidations)+1),
		Description: ruleDescription,
		FieldPaths:  fieldPaths,
	}
	v.validations = append(v.validations, &Validation{
		Rule:    rule,
		Options: options,
		Func:    validationFunction,
	})
}

func (v *Validator) AddAuthorizedRule(ruleDescription string, fieldPaths []string, validationFunction AuthorizedFunc, az *authz.Authz, options *Options) {
	v.validateFieldPaths(fieldPaths)
	v.addAuthorizedRule(ruleDescription, fieldPaths, validationFunction, az, options)
}

func (v *Validator) addAuthorizedRule(ruleDescription string, fieldPaths []string, validationFunction AuthorizedFunc, az *authz.Authz, options *Options) {
	if az == nil {
		alog.Fatalf(context.Background(), "cannot add authorized rule without authz")
	}
	rule := &pbOpen.Rule{
		Id:          fmt.Sprintf("%d", len(v.validations)+len(v.authorizedValidations)+1),
		Description: ruleDescription,
		FieldPaths:  fieldPaths,
	}
	v.authorizedValidations = append(v.authorizedValidations, &AuthorizedValidation{
		Rule:    rule,
		Options: options,
		Func:    validationFunction,
	})
}

func (v *Validator) HandleValidateRpc(req *pbOpen.ValidateMessageRequest) (*pbOpen.ValidateMessageResponse, error) {
	if v.msgType != req.MsgType {
		return nil, status.Errorf(codes.Internal, "msgType mismatch")
	}
	// clone proto message
	msg := proto.Clone(v.protoMsg)
	err := anypb.UnmarshalTo(req.Msg, msg, proto.UnmarshalOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal error: %v", err)
	}

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
	rules := make([]*pbOpen.Rule, len(v.validations)+len(v.authorizedValidations))
	for i, val := range v.validations {
		rules[i] = val.Rule
	}
	for i, val := range v.authorizedValidations {
		rules[i+len(v.validations)] = val.Rule
	}

	resp := &pbOpen.RetrieveRulesResponse{
		Rules: rules,
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
	msgType := GetMsgType(msg.(protoreflect.ProtoMessage))
	if msgType != v.msgType {
		return nil, status.Errorf(codes.Internal, "msgType mismatch")
	}
	var allViolations []*pbOpen.Violation
	alreadyViolatedFields := make(map[string]bool)
	fieldInfoCache := make(map[string]*FieldInfo)
	for _, val := range v.validations {
		if val.Options != nil {
			if val.Options.ConditionFunc != nil {
				conditionFieldInfoMap := make(map[string]*FieldInfo)
				for _, fieldPath := range val.Options.ConditionFields {
					fieldInfo, err := v.GetFieldInfo(msg, fieldPath, fieldInfoCache)
					if err != nil {
						return nil, err
					}
					conditionFieldInfoMap[fieldPath] = fieldInfo
				}
				evaluateRule, err := val.Options.ConditionFunc(msg, conditionFieldInfoMap)
				if err != nil {
					return nil, err
				}
				if !evaluateRule {
					continue
				}
			}
		}
		fieldInfoMap := make(map[string]*FieldInfo)
		for _, fieldPath := range val.Rule.FieldPaths {
			fieldInfo, err := v.GetFieldInfo(msg, fieldPath, fieldInfoCache)
			if err != nil {
				return nil, err
			}
			if val.Options != nil {
				if !val.Options.ValidateEmptyFields && !fieldInfo.Descriptor.HasPresence() {
					continue
				}
				if !val.Options.ValidateAlreadyViolatedFields {
					if _, ok := alreadyViolatedFields[fieldPath]; ok {
						continue
					}
				}
			}
			fieldInfoMap[fieldPath] = fieldInfo
		}
		if len(fieldInfoMap) == 0 {
			continue
		}
		violations, err := val.Func(msg, fieldInfoMap)
		if err != nil {
			return nil, err
		}
		for _, viol := range violations {
			viol.RuleId = val.Rule.Id
			alreadyViolatedFields[viol.FieldPath] = true
			allViolations = append(allViolations, viol)
		}
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

		for _, val := range v.authorizedValidations {
			if val.Options != nil {
				if val.Options.AuthorizedConditionFunc != nil {
					conditionFieldInfoMap := make(map[string]*FieldInfo)
					for _, fieldPath := range val.Options.ConditionFields {
						fieldInfo, err := v.GetFieldInfo(msg, fieldPath, fieldInfoCache)
						if err != nil {
							return nil, err
						}
						conditionFieldInfoMap[fieldPath] = fieldInfo
					}
					evaluateRule, err := val.Options.AuthorizedConditionFunc(msg, fieldInfoCache, authInfo)
					if err != nil {
						return nil, err
					}
					if !evaluateRule {
						continue
					}
				}
			}
			fieldInfoMap := make(map[string]*FieldInfo)
			for _, fieldPath := range val.Rule.FieldPaths {
				fieldInfo, err := v.GetFieldInfo(msg, fieldPath, fieldInfoCache)
				if err != nil {
					return nil, err
				}
				if val.Options != nil {
					if !val.Options.ValidateEmptyFields && !fieldInfo.Descriptor.HasPresence() {
						continue
					}
					if !val.Options.ValidateAlreadyViolatedFields {
						if _, ok := alreadyViolatedFields[fieldPath]; ok {
							continue
						}
					}
				}
				fieldInfoMap[fieldPath] = fieldInfo
			}
			if len(fieldInfoMap) == 0 {
				continue
			}
			violations, err := val.Func(msg, fieldInfoMap, authInfo)
			if err != nil {
				return nil, err
			}
			for _, viol := range violations {
				viol.RuleId = val.Rule.Id
				alreadyViolatedFields[viol.FieldPath] = true
				allViolations = append(allViolations, viol)
			}
		}
	}
	return allViolations, nil
}

func (v *Validator) GetFieldInfo(data interface{}, fieldPath string, fieldInfoCache map[string]*FieldInfo) (*FieldInfo, error) {
	if fieldInfoCache != nil {
		if val, ok := fieldInfoCache[fieldPath]; ok {
			return val, nil
		}
	}
	msg := data.(protoreflect.ProtoMessage)
	parentMsg, fieldName, err := GetFieldParentMessage(msg, fieldPath)
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
	println(fd.HasPresence())
	if fd.TextName() != fieldName {
		return nil, status.Errorf(codes.Internal, "%s not found", fieldName)
	}
	fieldInfo := FieldInfo{
		Descriptor: fd,
		Value:      parentMsg.ProtoReflect().Get(fd),
	}
	if fieldInfoCache != nil {
		fieldInfoCache[fieldPath] = &fieldInfo
	}
	return &fieldInfo, nil
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

func (v *Validator) GetStringField(data interface{}, fieldPath string) (string, error) {
	fieldInfo, err := v.GetFieldInfo(data, fieldPath, nil)
	if err != nil {
		return "", err
	}
	if fieldInfo.Descriptor.Kind() != protoreflect.StringKind {
		return "", status.Errorf(codes.Internal, "%s is not a string field", fieldPath)
	}
	return fieldInfo.Value.String(), nil
}

func (v *Validator) validateFieldPaths(fieldPaths []string, allowedTypes ...protoreflect.Kind) {
	for _, fieldPath := range fieldPaths {
		v.validateFieldPath(fieldPath, allowedTypes...)
	}
}

func (v *Validator) validateFieldPath(fieldPath string, allowedTypes ...protoreflect.Kind) {
	fi, err := v.GetFieldInfo(v.protoMsg, fieldPath, nil)
	if err != nil {
		alog.Fatalf(context.Background(), "field path (%s) not found for %s", fieldPath, v.msgType)
	}
	if len(allowedTypes) > 0 {
		if slices.Contains(allowedTypes, fi.Descriptor.Kind()) {
			alog.Fatalf(context.Background(), "field path (%s) is not a valid type for %s", fieldPath, v.msgType)
		}
	}
}
