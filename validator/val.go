package validator

import "google.golang.org/protobuf/proto"
import pb "internal.os.alis.services/protobuf/alis/os/processes/v1"

type Rule struct {
	Description string
	Function    func(*proto.Message) bool
}

type Val struct {
	Rules []*Rule
	// generic data
}

// new validator
func NewVal() *Val {
	return &Val{}
}

// add rule
func (v *Val) AddRule(r *Rule) *Val {
	v.Rules = append(v.Rules, r)
	return v
}

// validate
func (v *Val) Validate(data proto.Message) bool {
	for _, r := range v.Rules {
		if !r.Function(data) {
			return false
		}
	}
	return true
}

// example
func test(){
	v := NewVal()
	v.AddRule(&Rule{
		Description: "test",
		Function: func(process *pb.Process) bool {
			if process.Name ==""{
				return false
			}
			return true
		},
	}
}