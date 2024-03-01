package excel

import (
	"encoding/json"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
)

type CellValue interface {
	GetType() string
}

// EntityCellValue represents the Excel.EntityCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.entitycellvalue
type EntityCellValue struct {
	Type       string               `json:"type"`
	Text       string               `json:"text"`
	Properties map[string]CellValue `json:"properties"`
}

// FormattedNumber represents the Excel.FormattedNumber interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.formattednumbercellvalue
type formattedNumber struct {
	Type         string  `json:"type"`
	Value        float64 `json:"basicValue"`
	NumberFormat string  `json:"numberFormat"`
}

// StringCellValue represents the Excel.StringCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.stringcellvalue
type stringCellValue struct {
	Type  string `json:"type"`
	Value string `json:"basicValue"`
}

// BooleanCellValue The Excel.BooleanCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.booleancellvalue
type booleanCellValue struct {
	Type  string `json:"type"`
	Value bool   `json:"basicValue"`
}

// DoubleCellValue represents the Excel.DoubleCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.doublecellvalue
type doubleCellValue struct {
	Type  string  `json:"type"`
	Value float64 `json:"basicValue"`
}

// ArrayCellValue represents the Excel.ArrayCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.arraycellvalue
type arrayCellValue struct {
	Type     string        `json:"type"`
	Elements [][]CellValue `json:"elements"`
}

// GetType retrieves the basicType value from the Excel.Entity
func (f EntityCellValue) GetType() string { return f.Type }

// GetType retrieves the basicType value from the Excel.Entity
func (f formattedNumber) GetType() string { return f.Type }

// GetType retrieves the basicType value from the Excel.Entity
func (f stringCellValue) GetType() string { return f.Type }

// GetType retrieves the basicType value from the Excel.Entity
func (f booleanCellValue) GetType() string { return f.Type }

// GetType retrieves the basicType value from the Excel.Entity
func (f doubleCellValue) GetType() string { return f.Type }

// GetType retrieves the basicType value from the Excel.Entity
func (f arrayCellValue) GetType() string { return f.Type }

// BoolValue is a helper function to generate a BooleanCellValue object
func BoolValue(x bool) booleanCellValue {
	return booleanCellValue{
		Type:  "Boolean",
		Value: x,
	}
}

// BoolValue is a helper function to generate a BooleanCellValue object
func StringValue(x string) stringCellValue {
	return stringCellValue{
		Type:  "String",
		Value: x,
	}
}

// ArrayValue is a helper function to generate a ArrayCellValue object
func ArrayValue(elements [][]CellValue) arrayCellValue {
	return arrayCellValue{
		Type:     "Array",
		Elements: elements,
	}
}

// DoubleValue is a helper function to generate a DoubleCellValue object
func DoubleValue(x float64) doubleCellValue {
	return doubleCellValue{
		Type:  "Double",
		Value: x,
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

// FormattedValue is a helper function to generate a FormattedCellValue object
func FormattedValue(x float64, format string) formattedNumber {
	return formattedNumber{
		Type:         "FormattedNumber",
		Value:        x,
		NumberFormat: format,
	}
}

// NewCell is a helper function to generate a new EntityCellValue object, i.e. and Excel Card.
func NewCell(text string, properties map[string]CellValue) EntityCellValue {
	return EntityCellValue{
		Type:       "Entity",
		Text:       text,
		Properties: properties,
	}
}

// ToJSON marshals the EntiteyCellValue object to a JSON object ready for use by MS Excel.
func (e *EntityCellValue) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}
