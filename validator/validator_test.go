package validator

import (
	"testing"

	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func init() {
	val := NewValidator(&pbOpen.Test{})
	// val.AddRule("my custom rule", []string{"name", "int32"}, func(data interface{}, fields map[string]*FieldInfo) ([]*pbOpen.Violation, error) {
	// 	violations := make([]*pbOpen.Violation, 0)
	// 	if fields["name"].Value.String() == "asdf" {
	// 		if fields["int32"].Value.Int() == 0 {
	// 			violations = append(violations, &pbOpen.Violation{
	// 				FieldPath: "int32",
	// 				Message:   "int32 must be greater than 0 if name is asdf",
	// 			})
	// 		}
	// 	}
	// 	return violations, nil
	// })
	val.AddRequiredFieldsRule([]string{"name", "int32"}, NewNotCondition(val.NewIsPopulatedCondition("int64")))
	val.AddRegexRule("name", "^organisations/[a-z][a-z0-9]{2,9}/products/[a-z]{2}$")
	val.AddListLengthRule("repeated_string", 2, 3, val.NewEnumCondition("test_enum", pbOpen.TestEnum_TEST_ENUM_ONE), val.NewIsPopulatedCondition("int64"))
}

func TestNewValidator(t *testing.T) {
	// msg := &pbOpen.Test{RepeatedString: []string{"asdf"}, TestEnum: pbOpen.TestEnum_TEST_ENUM_ONE, Int64: 10}
	// err, found := Validate(msg)
	// if err != nil {
	// 	t.Logf("unexpected error: %v", err)
	// }
	// t.Logf("found: %v", found)

	// convert msg to any type

	rulesResp, err := RetrieveRulesRpc(&pbOpen.RetrieveRulesRequest{MsgType: "alis.open.validation.v1.Test"})
	if err != nil {
		t.Logf("unexpected error: %v", err)
	}
	t.Logf("rulesResp: %v", rulesResp)
}
