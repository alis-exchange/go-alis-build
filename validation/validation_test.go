package validation_test

import (
	"testing"

	"go.alis.build/validation"
)

func Test_ValidateBasic(t *testing.T) {
	v := validation.NewValidator()
	v.String("name", "John").Populated()
	err := v.Error()
	if err != nil {
		t.Error(err)
	}
}

func Test_ValidateOr(t *testing.T) {
	v := validation.NewValidator()
	v.Or(v.String("name", "").Populated(), v.String("email", "").Populated())
	err := v.Error()
	if err != nil {
		t.Error(err)
	}
}

func Test_ValidateIf(t *testing.T) {
	v := validation.NewValidator()
	v.If(v.Int32("age", 17).Gt(18)).Then(
		v.String("name", "").Populated(),
	)
	err := v.Error()
	if err != nil {
		t.Error(err)
	}
}
