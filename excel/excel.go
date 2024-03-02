package excel

import (
	"encoding/json"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
)

type CellValue interface {
	ToJSON() ([]byte, error)
}

// EntityCellValue represents the Excel.EntityCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.entitycellvalue
type entityCellValue struct {
	Type       string               `json:"type"`
	Text       string               `json:"text"`
	Properties map[string]CellValue `json:"properties"`
}

// Entity is a helper function to generate a new EntityCellValue object, i.e. an Excel Card.
func EntityValue(text string, properties map[string]CellValue) entityCellValue {
	return entityCellValue{
		Type:       "Entity",
		Text:       text,
		Properties: properties,
	}
}

// ToJSON marshals the CellValue object to a JSON object ready for use by MS Excel.
func (e entityCellValue) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// FormattedNumber represents the Excel.FormattedNumber interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.formattednumbercellvalue
type formattedNumber struct {
	Type         string  `json:"type"`
	Value        float64 `json:"basicValue"`
	NumberFormat string  `json:"numberFormat"`
}

// FormattedNumber is a helper function to generate a FormattedCellValue object
func FormattedNumber(x float64, format string) formattedNumber {
	return formattedNumber{
		Type:         "FormattedNumber",
		Value:        x,
		NumberFormat: format,
	}
}

// DateValue is a helper function to generate a Date cell with a specified format
func DateValue(x *date.Date, format string) formattedNumber {
	// First convert the Date object to number of days since 1900
	baseTime := time.Date(1900, 1, -1, 0, 0, 0, 0, time.UTC)
	xTime := time.Date(int(x.GetYear()), time.Month(x.GetMonth()), int(x.GetDay()), 0, 0, 0, 0, time.UTC)

	// Calculate duration between dates
	duration := xTime.Sub(baseTime)

	// Return days since 1900
	days := int64(duration.Hours() / 24)

	return formattedNumber{
		Type:         "FormattedNumber",
		Value:        float64(days),
		NumberFormat: format,
	}
}

// ToJSON marshals the CellValue object to a JSON object ready for use by MS Excel.
func (f formattedNumber) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// StringCellValue represents the Excel.StringCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.stringcellvalue
type stringCellValue struct {
	Type  string `json:"type"`
	Value string `json:"basicValue"`
}

// BoolValue is a helper function to generate a BooleanCellValue object
func StringValue(x string) stringCellValue {
	return stringCellValue{
		Type:  "String",
		Value: x,
	}
}

// ToJSON marshals the CellValue object to a JSON object ready for use by MS Excel.
func (s stringCellValue) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// BooleanCellValue The Excel.BooleanCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.booleancellvalue
type booleanCellValue struct {
	Type  string `json:"type"`
	Value bool   `json:"basicValue"`
}

// BoolValue is a helper function to generate a BooleanCellValue object
func BoolValue(x bool) booleanCellValue {
	return booleanCellValue{
		Type:  "Boolean",
		Value: x,
	}
}

// ToJSON marshals the CellValue object to a JSON object ready for use by MS Excel.
func (b booleanCellValue) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// DoubleCellValue represents the Excel.DoubleCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.doublecellvalue
type doubleCellValue struct {
	Type  string  `json:"type"`
	Value float64 `json:"basicValue"`
}

// DoubleValue is a helper function to generate a DoubleCellValue object
func DoubleValue(x float64) doubleCellValue {
	return doubleCellValue{
		Type:  "Double",
		Value: x,
	}
}

// ToJSON marshals the CellValue object to a JSON object ready for use by MS Excel.
func (d doubleCellValue) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// ArrayCellValue represents the Excel.ArrayCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.arraycellvalue
type arrayCellValue struct {
	Type     string        `json:"type"`
	Elements [][]CellValue `json:"elements"`
}

// ArrayValue is a helper function to generate a ArrayCellValue object
func ArrayValue(elements [][]CellValue) arrayCellValue {
	return arrayCellValue{
		Type:     "Array",
		Elements: elements,
	}
}

// ToJSON marshals the CellValue object to a JSON object ready for use by MS Excel.
func (a arrayCellValue) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}
