package validator

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

// Helper function to implement your Validate rpc
func HandleValidateRpc(ctx context.Context, req *pbOpen.ValidateMessageRequest) (*pbOpen.ValidateMessageResponse, error) {
	v, err := getValidator(ctx, req.MsgType)
	if err != nil {
		return nil, err
	}
	msgBytes := req.Msg
	// clone v.protoMsg
	msg := proto.Clone(v.protoMsg)
	err = proto.Unmarshal(msgBytes, msg)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal message into %s", v.msgType)
	}
	viols, err := v.GetViolations(msg, req.FieldPaths, []string{})
	if err != nil {
		return nil, err
	}
	violatedFieldsMap := map[string][]*pbOpen.Rule{}
	for _, viol := range viols {
		for _, path := range viol.FieldPaths {
			if _, ok := violatedFieldsMap[path]; !ok {
				violatedFieldsMap[path] = []*pbOpen.Rule{}
			}
			violatedFieldsMap[path] = append(violatedFieldsMap[path], viol)
		}
	}
	violatedFields := []*pbOpen.FieldViolation{}
	for k, v := range violatedFieldsMap {
		violatedFields = append(violatedFields, &pbOpen.FieldViolation{
			FieldPath:     k,
			ViolatedRules: v,
		})
	}
	return &pbOpen.ValidateMessageResponse{
		ViolatedRules:  viols,
		ViolatedFields: violatedFields,
	}, nil
}

// Helper function to implement your RetrieveRules rpc
func RetrieveRulesRpc(ctx context.Context, req *pbOpen.RetrieveRulesRequest) (*pbOpen.RetrieveRulesResponse, error) {
	v, err := getValidator(ctx, req.MsgType)
	if err != nil {
		return nil, err
	}
	rules := v.GetRules()
	return &pbOpen.RetrieveRulesResponse{
		Rules: rules,
	}, nil
}
