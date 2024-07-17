package validator

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

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
		rules[i] = val.rule
		rules[i].Conditions = make([]string, len(val.conditions))
		for j, cond := range val.conditions {
			rules[i].Conditions[j] = cond.GetDescription()
		}
	}
	for i, val := range v.authorizedValidations {
		rules[i+len(v.validations)] = val.rule
		rules[i+len(v.validations)].Conditions = make([]string, len(val.conditions))
		for j, cond := range val.conditions {
			rules[i+len(v.validations)].Conditions[j] = cond.GetDescription()
		}
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
