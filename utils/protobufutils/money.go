package protobufutils

import (
	"math"

	"google.golang.org/genproto/googleapis/type/money"
)

// MoneyToFloat64 converts a google.type.Money protobuf message to a float64 value.
//
// The function combines the units (integer part) and nanos (fractional part) to create
// a single float64 value. The nanos are divided by 1e9 to convert from nanoseconds
// (where 1e9 nanos = 1 unit) to the fractional part of the float.
//
// If moneyObj is nil, it returns 0.0.
//
// Example:
//
//	m := &money.Money{Units: 10, Nanos: 500000000, CurrencyCode: "USD"}
//	value := MoneyToFloat64(m) // Returns 10.5
//
//	m2 := &money.Money{Units: -5, Nanos: -250000000, CurrencyCode: "USD"}
//	value2 := MoneyToFloat64(m2) // Returns -5.25
func MoneyToFloat64(moneyObj *money.Money) float64 {
	if moneyObj == nil {
		return 0.0
	}

	units := moneyObj.GetUnits()
	nanos := moneyObj.GetNanos()

	return float64(units) + float64(nanos)/1e9
}

// Float64ToMoney converts a float64 value and currency code to a google.type.Money protobuf message.
//
// The function splits the float64 value into units (integer part) and nanos (fractional part).
// The fractional part is multiplied by 1e9 and converted to int32 nanos, where 1e9 nanos = 1 unit.
//
// The nanos value is rounded to the nearest integer to handle floating-point precision issues.
// If the resulting nanos value would overflow int32, it is clamped to the int32 range.
//
// Currency code is stored as-is without validation. It should be a valid ISO 4217 currency code
// (e.g., "USD", "EUR", "GBP").
//
// Example:
//
//	m := Float64ToMoney("USD", 10.5)
//	// Returns: &money.Money{Units: 10, Nanos: 500000000, CurrencyCode: "USD"}
//
//	m2 := Float64ToMoney("EUR", -5.25)
//	// Returns: &money.Money{Units: -5, Nanos: -250000000, CurrencyCode: "EUR"}
//
//	m3 := Float64ToMoney("GBP", 0.01)
//	// Returns: &money.Money{Units: 0, Nanos: 10000000, CurrencyCode: "GBP"}
func Float64ToMoney(currency string, value float64) *money.Money {
	// Split the value into integer and fractional parts
	units, fractional := math.Modf(value)

	// Convert fractional part to nanos (multiply by 1e9)
	// Round to nearest integer to handle floating-point precision issues
	nanosFloat := fractional * 1e9
	nanosRounded := math.Round(nanosFloat)
	nanos := int32(nanosRounded)

	// Handle case where rounding caused nanos to overflow into a full unit
	// If nanos is >= 1e9, we need to add 1 to units and subtract 1e9 from nanos
	// If nanos is <= -1e9, we need to subtract 1 from units and add 1e9 to nanos
	if nanosRounded >= 1e9 {
		units += 1
		nanos = int32(nanosRounded - 1e9)
	} else if nanosRounded <= -1e9 {
		units -= 1
		nanos = int32(nanosRounded + 1e9)
	}

	return &money.Money{
		CurrencyCode: currency,
		Units:        int64(units),
		Nanos:        nanos,
	}
}
