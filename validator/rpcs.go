package validator

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func HandleValidateRpc(req *pbOpen.ValidateMessageRequest) (*pbOpen.ValidateMessageResponse, error) {
	v, ok := validators[req.MsgType]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "msgType not found")
	}
	msgBytes := req.Msg
	// clone v.protoMsg
	msg := proto.Clone(v.protoMsg)
	err := proto.Unmarshal(msgBytes, msg)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal message into %s", v.msgType)
	}
	viols, err := v.GetViolations(msg, []string{})
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
