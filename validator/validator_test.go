package validator

import (
	"testing"

	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func init() {
	val := NewValidator(&pbOpen.Test{})
	val.AddRequiredFieldsRule([]string{"name", "int32"}).ApplyIf(EnumIs("test_enum", pbOpen.TestEnum_TEST_ENUM_ONE))
	val.AddRequiredFieldsRule([]string{"int64"})
}

func TestNewValidator(t *testing.T) {
	msg := &pbOpen.Test{RepeatedString: []string{"asdf"}, TestEnum: pbOpen.TestEnum_TEST_ENUM_ONE, Int64: 10}
	err, found := Validate(msg)
	if err != nil {
		t.Logf("unexpected error: %v", err)
	}
	t.Logf("found: %v", found)

	// convert msg to any type

	//		rulesResp, err := RetrieveRulesRpc(&pbOpen.RetrieveRulesRequest{MsgType: "alis.open.validation.v1.Test"})
	//		if err != nil {
	//			t.Logf("unexpected error: %v", err)
	//		}
	//		t.Logf("rulesResp: %v", rulesResp)
	//	}
}
