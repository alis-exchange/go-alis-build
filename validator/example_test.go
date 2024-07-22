package validator_test

import (
	"context"
	"fmt"

	"go.alis.build/alog"
	"go.alis.build/validator"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

var (
	StringField      = validator.StringField
	String           = validator.String
	EachStringIn     = validator.EachStringIn
	IntField         = validator.IntField
	Int              = validator.Int
	FloatField       = validator.FloatField
	Float            = validator.Float
	EnumField        = validator.EnumField
	Enum             = validator.Enum
	SubMsgListLength = validator.SubMsgListLength
	Now              = validator.Now
	TimestampField   = validator.TimestampField
	DateField        = validator.DateField
	FieldMaskField   = validator.FieldMaskField
	AND              = validator.AND
	OR               = validator.OR
	NOT              = validator.NOT
)

var testBook = &pbOpen.Book{
	DisplayName:     "The Book",
	Type:            pbOpen.Type_ANONYMOUS,
	PublicationDate: &date.Date{Year: 2021, Month: 1, Day: 1},
	// UpdateTime:      timestamppb.Now(),
}

var createBook = &pbOpen.CreateBookRequest{
	Book: testBook,
}
var createBookRequest = createBook.ProtoReflect().Interface()

var updateBook = &pbOpen.UpdateBookRequest{
	Book:       testBook,
	UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name", "name"}},
}
var updateBookRequest = updateBook.ProtoReflect().Interface()

func init() {
	bookVal := validator.NewValidator(&pbOpen.Book{}, &validator.ValidatorOptions{IgnoreWarnings: true})
	bookVal.AddRule(StringField("display_name").Length().InRange(Int(3), Int(63)))
	bookVal.AddRule(EnumField("type").Populated())
	bookVal.AddRule(AND(
		StringField("author").MatchesRegex(String("^[a-zA-Z0-9_]*$")),
		StringField("author").Length().InRange(Int(3), Int(63)),
	)).ApplyIf(NOT(
		EnumField("type").Equals(Enum(pbOpen.Type_ANONYMOUS)),
	))
	bookVal.AddRule(Now().After(DateField("publication_date")))
	bookVal.AddRule(NOT(TimestampField("update_time").Populated()))

	createBookVal := validator.NewValidator(&pbOpen.CreateBookRequest{}, &validator.ValidatorOptions{IgnoreWarnings: true})
	createBookVal.AddSubMessageValidator("book", bookVal, &validator.SubMsgOptions{})

	updateBookVal := validator.NewValidator(&pbOpen.UpdateBookRequest{}, &validator.ValidatorOptions{IgnoreWarnings: true})
	updateBookVal.AddRule(FieldMaskField("update_mask").OnlyContains([]string{"display_name"}))
	updateBookVal.AddSubMessageValidator("book", bookVal, &validator.SubMsgOptions{OnlyValidateFieldsSpecifiedIn: "update_mask"})
}

func ExampleValidate() {
	err, found := validator.Validate(updateBookRequest)
	if err != nil {
		fmt.Printf("Update book validator errors: %v\n", err)
	}
	if !found {
		alog.Warn(context.Background(), "No validator found for request")
	}
}

// get create book rules
func ExampleRetrieveRulesRpc() {
	retrieveCreateBookRulesReq := &pbOpen.RetrieveRulesRequest{
		MsgType: "alis.open.validation.v1.CreateBookRequest",
	}
	resp, err := validator.RetrieveRulesRpc(retrieveCreateBookRulesReq)
	if err != nil {
		fmt.Printf("Retrieve create book rules errors: %v\n", err)
	}
	fmt.Printf("Retrieve create book rules response: %v\n", resp)
}
