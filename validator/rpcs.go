package validator

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func HandleValidateRpc(req *pbOpen.ValidateMessageRequest) (*pbOpen.ValidateMessageResponse, error) {
	v, ok := validators[req.MsgType]
	if !ok {
		return nil, status.Errorf(codes.Internal, "msgType not found")
	}
	viols, err := v.GetViolations(req)
	if err != nil {
		return nil, err
	}
	return &pbOpen.ValidateMessageResponse{
		Violations: viols,
	}, nil
}

func RetrieveRulesRpc(req *pbOpen.RetrieveRulesRequest) (*pbOpen.RetrieveRulesResponse, error) {
	v, ok := validators[req.MsgType]
	if !ok {
		return nil, status.Errorf(codes.Internal, "msgType not found")
	}
	rules := v.GetRules()
	return &pbOpen.RetrieveRulesResponse{
		Rules: rules,
	}, nil
}
