package excel

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
)

//go:embed script_lab.tmpl
var templateFs embed.FS

type CellValue interface {
	ToJSON() ([]byte, error)
	ToScriptLabYAML() (string, error)
}

// EntityCellValue represents the Excel.EntityCellValue interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.entitycellvalue
type entityCellValue struct {
	Type       string               `json:"type"`                 // Represents the type of this cell value
	Text       string               `json:"text"`                 // Represents the text shown when a cell with this value is rendered.
	Properties map[string]CellValue `json:"properties,omitempty"` // Represents the properties of this entity and their metadata.
	Layouts    *Layouts             `json:"layouts,omitempty"`    // Represents layout information for views of this entity.
	Provider   *Provider            `json:"provider,omitempty"`   // Represents information that describes the service that provided the data in this EntityCellValue. This information can be used for branding in entity cards.
}

// entityViewLayouts represents layout information for various views of the entity.
// Defined at: https://learn.microsoft.com/en-us/javascript/api/excel/excel.entityviewlayouts
type Layouts struct {
	// Represents the layout used when there is limited space to represent the entity.
	Compact *Compact `json:"compact,omitempty"`
	// Represents the layout of this entity in card view.
	Card *Card `json:"card,omitempty"`
}

// Compact represents the compact layout properties for an entity, as defined at:
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.entitycompactlayout
type Compact struct {
	// Specifies the name of the icon which is used to open the card.
	// Examples: Airplane, Alert, Code, Cloud, ShoppingBag, etc.
	// Full list of icons available at https://learn.microsoft.com/en-us/javascript/api/excel/excel.entitycompactlayouticons
	Icon string `json:"icon,omitempty"`
}

// Card represents the layout of a card in card view.
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.cardlayout
// TODO: implement EntityArrayCardLayout version of the CardLayout type.
type Card struct {
	// Represents the title of the card or the specification of which property contains the title of the card.
	Title *CardProperty `json:"title,omitempty"`
	// Represents a specification of which property contains the subtitle of the card.
	SubTitle *CardProperty `json:"subTitle,omitempty"`
	// Specifies a property which will be used as the main image of the card.
	MainImage *CardProperty `json:"mainImage,omitempty"`
	// Represents the sections of the card.
	Sections []Section `json:"sections,omitempty"`
}

// Represents the layout of a section of a card in card view.
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.cardlayoutsection
type Section struct {
	// Represents the type of layout for this section.
	// Available values: "List", "Table"
	Layout string `json:"layout,omitempty"`
	// Represents the names of the properties in this section.
	Properties []string `json:"properties,omitempty"`
	// Represents the title of this section of the card.
	Title *string `json:"title,omitempty"`
	// Represents whether this section of the card is collapsible. If the card section
	// has a title, the default value is true. If the card section doesn't have a title,
	// the default value is false.
	Collapsible *bool `json:"collapsible,omitempty"`
	// Represents whether this section of the card is initially collapsed.
	Collapsed *bool `json:"collapsed,omitempty"`
}

type CardProperty struct {
	// The key of the relevant property within the Properties map attribute
	Property string `json:"property,omitempty"`
}

// CellValueProviderAttributes as defined at:
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.cellvalueproviderattributes
type Provider struct {
	// Represents the provider description property that is used in card view if no logo is specified.
	// If a logo is specified, this will be used as tooltip text.

	Description string `json:"description,omitempty"`
	// Represents a URL used to download an image that will be used as a logo in card view.
	LogoSourceAddress string `json:"logoSourceAddress,omitempty"`
	// Represents a URL that is the navigation target if the user clicks on the logo element in card view
	LogoTargetAddress string `json:"logoTargetAddress,omitempty"`
}

// Entity is a helper function to generate a new EntityCellValue object, i.e. an Excel Card.
func EntityValue(text string, properties map[string]CellValue, layouts *Layouts, provider *Provider) entityCellValue {
	return entityCellValue{
		Type:       "Entity",
		Text:       text,
		Properties: properties,
		Layouts:    layouts,
		Provider:   provider,
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

/*
ToScriptLabYAML function streamlines the process of validating your generated Entity objects. Here's how it works:

# Getting Started:
  - Install Script Lab: Enhance Excel with this Microsoft Garage add-in. Find it by going to Home -> Add-ins.
  - Open a New Script: Navigate to Script Lab -> Code.
  - Effortless Import: Copy and paste the text output from ToScriptLabImport into the Import section of your script.

# Key Benefits:
  - Fast and Easy Validation: Quickly confirm that your generated Entity objects function correctly within your Excel environment.
  - Enhanced Workflow: This streamlined process saves you time and effort.
*/
func (a entityCellValue) ToScriptLabYAML() (string, error) {
	return parseImportYAML(a)
}

// parseImportYAML is a generic helper method using the '.ToJSON()' method from relevant CellValue types
func parseImportYAML(c CellValue) (string, error) {
	// Retrieve JSON version of the entity
	entityCellValueJson, err := c.ToJSON()
	if err != nil {
		return "", fmt.Errorf("generating json: %w", err)
	}

	// We'll use the built in template package to parse the script file.
	var scriptBytes bytes.Buffer
	fileTemplate, err := templateFs.ReadFile("script_lab.tmpl")
	if err != nil {
		return "", fmt.Errorf("read script_lab.tmpl file: %w", err)
	}
	t, err := template.New("scriptlab").Parse(string(fileTemplate))
	if err != nil {
		return "", fmt.Errorf("create parser: %w", err)
	}

	err = t.Execute(&scriptBytes, struct{ EntityCellValueJson string }{
		EntityCellValueJson: string(entityCellValueJson),
	})
	if err != nil {
		return "", fmt.Errorf("parse script_lab.tmpl file: %w", err)
	}

	return scriptBytes.String(), nil
}

// FormattedNumber represents the Excel.FormattedNumber interface as defined at
// https://learn.microsoft.com/en-us/javascript/api/excel/excel.formattednumbercellvalue
type formattedNumber struct {
	Type         string  `json:"type"`
	Value        float64 `json:"basicValue"`
	NumberFormat string  `json:"numberFormat"`
}

// FormattedNumber is a helper function to generate a FormattedCellValue object.
//
// Format examples, for the value 1234.56
//   - $0.00 -> $1234.56
//   - $# ##0.00 -> $1 234.56
//   - $ # -> $ 1234
//   - "[Blue]#,##0.00_);[Red](#,##0.00);0.00;" -> 1,234.56 (in blue)
//
// https://support.microsoft.com/en-us/office/review-guidelines-for-customizing-a-number-format-c0a1d1fa-d3f4-4018-96b7-9c9354dd99f5
func FormattedNumber(x float64, format string) formattedNumber {
	return formattedNumber{
		Type:         "FormattedNumber",
		Value:        x,
		NumberFormat: format,
	}
}

// DateValue is a helper function to generate a Date cell with a specified format.
//
// Format examples, for the value 31 January 1980
//   - yy-mm-dd -> 80-01-31
//   - yyyy-mm-dd -> 1980-01-31
//   - d mmm yyyy -> 31 Jan 1980
//   - d mmmm yyyy, dddd -> 31 January 1980, Saturday
//
// https://support.microsoft.com/en-us/office/review-guidelines-for-customizing-a-number-format-c0a1d1fa-d3f4-4018-96b7-9c9354dd99f5
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

/*
ToScriptLabYAML function streamlines the process of validating your generated Entity objects. Here's how it works:

# Getting Started:
  - Install Script Lab: Enhance Excel with this Microsoft Garage add-in. Find it by going to Home -> Add-ins.
  - Open a New Script: Navigate to Script Lab -> Code.
  - Effortless Import: Copy and paste the text output from ToScriptLabImport into the Import section of your script.

# Key Benefits:
  - Fast and Easy Validation: Quickly confirm that your generated Entity objects function correctly within your Excel environment.
  - Enhanced Workflow: This streamlined process saves you time and effort.
*/
func (f formattedNumber) ToScriptLabYAML() (string, error) {
	return parseImportYAML(f)
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

/*
ToScriptLabYAML function streamlines the process of validating your generated Entity objects. Here's how it works:

# Getting Started:
  - Install Script Lab: Enhance Excel with this Microsoft Garage add-in. Find it by going to Home -> Add-ins.
  - Open a New Script: Navigate to Script Lab -> Code.
  - Effortless Import: Copy and paste the text output from ToScriptLabImport into the Import section of your script.

# Key Benefits:
  - Fast and Easy Validation: Quickly confirm that your generated Entity objects function correctly within your Excel environment.
  - Enhanced Workflow: This streamlined process saves you time and effort.
*/
func (s stringCellValue) ToScriptLabYAML() (string, error) {
	return parseImportYAML(s)
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

/*
ToScriptLabYAML function streamlines the process of validating your generated Entity objects. Here's how it works:

# Getting Started:
  - Install Script Lab: Enhance Excel with this Microsoft Garage add-in. Find it by going to Home -> Add-ins.
  - Open a New Script: Navigate to Script Lab -> Code.
  - Effortless Import: Copy and paste the text output from ToScriptLabImport into the Import section of your script.

# Key Benefits:
  - Fast and Easy Validation: Quickly confirm that your generated Entity objects function correctly within your Excel environment.
  - Enhanced Workflow: This streamlined process saves you time and effort.
*/
func (b booleanCellValue) ToScriptLabYAML() (string, error) {
	return parseImportYAML(b)
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

/*
ToScriptLabYAML function streamlines the process of validating your generated Entity objects. Here's how it works:

# Getting Started:
  - Install Script Lab: Enhance Excel with this Microsoft Garage add-in. Find it by going to Home -> Add-ins.
  - Open a New Script: Navigate to Script Lab -> Code.
  - Effortless Import: Copy and paste the text output from ToScriptLabImport into the Import section of your script.

# Key Benefits:
  - Fast and Easy Validation: Quickly confirm that your generated Entity objects function correctly within your Excel environment.
  - Enhanced Workflow: This streamlined process saves you time and effort.
*/
func (d doubleCellValue) ToScriptLabYAML() (string, error) {
	return parseImportYAML(d)
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

/*
ToScriptLabYAML function streamlines the process of validating your generated Entity objects. Here's how it works:

# Getting Started:
  - Install Script Lab: Enhance Excel with this Microsoft Garage add-in. Find it by going to Home -> Add-ins.
  - Open a New Script: Navigate to Script Lab -> Code.
  - Effortless Import: Copy and paste the text output from ToScriptLabImport into the Import section of your script.

# Key Benefits:
  - Fast and Easy Validation: Quickly confirm that your generated Entity objects function correctly within your Excel environment.
  - Enhanced Workflow: This streamlined process saves you time and effort.
*/
func (a arrayCellValue) ToScriptLabYAML() (string, error) {
	return parseImportYAML(a)
}
