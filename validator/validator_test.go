package validator

import (
	"testing"

	pbProducts "internal.os.alis.services/protobuf/alis/os/resources/products/v1"
)

func TestNewValidator(t *testing.T) {
	val := NewValidator(&pbProducts.Product{})

	val.AddRequiredFieldsRule([]string{"name"}, nil)
	// val.AddRegexRule("name", "^organisations/[a-z][a-z0-9]{2,9}/products/[a-z]{2}$", nil)
	// val.AddIntRangeRule("update_time.seconds", 10, 9999999999, nil)
	val.AddEmailRule("name")

	// violations, err := val.GetViolations(&pbProducts.Product{Name: "organisations/alis/products/ab"}, false)
	// if err != nil {
	// 	t.Logf("unexpected error: %v", err)
	// }
	// t.Logf("violations: %v", violations)
	// err = val.Validate(&pbProducts.Product{Name: "organisations/alis/products/ab"})
	// if err != nil {
	// 	t.Logf("unexpected error: %v", err)
	// }
	err, found := Validate(&pbProducts.Product{Name: "daniel@alisx.com"})
	if err != nil {
		t.Logf("unexpected error: %v", err)
	}
	t.Logf("found: %v", found)
}
