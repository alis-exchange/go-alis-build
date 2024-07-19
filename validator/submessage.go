package validator

// import "google.golang.org/protobuf/reflect/protoreflect"

// // type submsg struct {
// // 	// description string
// // 	path string
// // 	// getValue    func(v *Validator, msg protoreflect.ProtoMessage) protoreflect.ProtoMessage
// // 	v *Validator
// // }

// type SubMessage struct {
// 	path string
// 	v    *Validator
// }

// func SubMessageRules(path string, val *Validator) *SubMessage {
// 	return &SubMessage{path: path, v: val}
// }

// func (s *SubMessage) ApplyAll() []*Rule {
// 	rules := []*Rule{}
// 	for _, r := range s.v.rules {
// 		subR := NewRule(&Rule{
// 			Id: "todo",
// 			isViolated: func(msg protoreflect.ProtoMessage) (bool, error) {

// 			},
// 		})
// 	}
// }
