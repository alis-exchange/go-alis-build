package excel_test

import (
	"go.alis.build/excel"
	"google.golang.org/genproto/googleapis/type/date"
)

func ExampleEntityValue() {
	e := excel.EntityValue(
		"Card Top Title",
		map[string]excel.CellValue{
			"Total Amount": excel.DoubleValue(7777.77),
			"Price":        excel.FormattedNumber(55.40, "$0.00"),
			"Validated":    excel.BoolValue(true),
			"Owner":        excel.StringValue("Jan Krynauw"),
			"Items": excel.ArrayValue([][]excel.CellValue{
				{
					excel.StringValue("Thomas"),
					excel.StringValue("Scholtz"),
					excel.FormattedNumber(8.03, "$0.0"),
				},
				{
					excel.StringValue("James"),
					excel.StringValue("Spanjaard"),
					excel.FormattedNumber(28.3, "$0.0"),
				},
			}),
			"Effective Date": excel.DateValue(&date.Date{
				Year:  1980,
				Month: 2,
				Day:   2,
			}, "yyyy-mm-dd"),
			"Sub Properties A": excel.EntityValue(
				"Another one",
				map[string]excel.CellValue{
					"Key 1": excel.StringValue("Value 1"),
					"Key 2": excel.StringValue("Value 2"),
				},
				nil, nil),
		},
		&excel.Layouts{
			Compact: &excel.Compact{
				Icon: "Cloud",
			},
			Card: &excel.Card{
				Title: &excel.CardProperty{
					Property: "Owner",
				},
				SubTitle: &excel.CardProperty{
					Property: "Effective Date",
				},
			},
		},
		&excel.Provider{
			Description:       "Some nice description",
			LogoTargetAddress: "",
		},
	)

	// Generate a JSON
	jsonBytes, _ := e.ToJSON()
	_ = jsonBytes

	// Generate ScriptLabImportYAML
	importXML, _ := e.ToScriptLabYAML()
	_ = importXML
}
