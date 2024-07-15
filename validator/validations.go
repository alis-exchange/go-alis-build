package validator

import (
	"google.golang.org/protobuf/proto"
)

type bufProtoValidate struct{}

func (c bufProtoValidate) Do(data proto.Message) []Violation {
	// We'll use the github.com/bufbuild/protovalidate-go pacakge to validate a message
	// For each of the provided fields, ensure they are populated
	violations := []Violation{
		{
			Message: "Testing buf protovalidate",
		},
	}
	return violations
}

// BufProtoValidate uses the Buf proto options to create a set of validations.
// This method makes use of the we'll use the github.com/bufbuild/protovalidate-go pacakge
// to validate a message
func BufProtoValidate() bufProtoValidate {
	return bufProtoValidate{}
}
