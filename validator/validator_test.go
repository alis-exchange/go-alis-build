package validator

import (
	"time"

	"go.alis.build/validator"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func init() {
	F
	Str,Float,Int,Bool,Enum,Timestamp
	val := NewValidator(&pbOpen.Test{})
	val.AddRule(validator.RegexMatches(Field("name"), "^[a-zA-Z0-9]*$"))
	cond := OR(validator.IsPopulated(Field("overview")), validator.IsPopulated(Field("description")))
	otherCond := validator.EnumIsNot(Field("state"), EnumValue(pbOpen.State_ARCHIVED))
	val.AddRule(cond).ApplyIf(otherCond)
	sumResult := validator.Sum(Field("amount"), Field("tax"), Float(100))
	val.AddRule(sumResult.LessThan(Float(200)))
	val.IsBefore(F("start_time"),Now())
}
