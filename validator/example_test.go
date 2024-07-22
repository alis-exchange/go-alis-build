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

func init() {
	ExampleValidator()
}

func ExampleValidator() {
	var (
		StringField    = validator.StringField
		String         = validator.String
		Int            = validator.Int
		EnumField      = validator.EnumField
		Enum           = validator.Enum
		Now            = validator.Now
		TimestampField = validator.TimestampField
		DateField      = validator.DateField
		FieldMaskField = validator.FieldMaskField
		AND            = validator.AND
		NOT            = validator.NOT
	)

	bookVal := validator.NewValidator(&pbOpen.Book{}, &validator.ValidatorOptions{IgnoreWarnings: true})
	// length of display_name should be between 3 and 63
	bookVal.AddRule(StringField("display_name").Length().InRange(Int(3), Int(63)))
	// type should be populated
	bookVal.AddRule(EnumField("type").Populated())
	// if type is not ANONYMOUS, author should match regex and length should be between 3 and 63
	bookVal.AddRule(AND(
		StringField("author").MatchesRegex(String("^[a-zA-Z0-9_]*$")),
		StringField("author").Length().InRange(Int(3), Int(63)),
	)).ApplyIf(NOT(
		EnumField("type").Equals(Enum(pbOpen.Type_ANONYMOUS)),
	))
	// publication_date should be in the past
	bookVal.AddRule(Now().After(DateField("publication_date")))
	// update_time should not be populated
	bookVal.AddRule(NOT(TimestampField("update_time").Populated()))

	createBookVal := validator.NewValidator(&pbOpen.CreateBookRequest{}, &validator.ValidatorOptions{IgnoreWarnings: true})
	createBookVal.AddSubMessageValidator("book", bookVal, &validator.SubMsgOptions{})

	updateBookVal := validator.NewValidator(&pbOpen.UpdateBookRequest{}, &validator.ValidatorOptions{IgnoreWarnings: true})
	// update_mask should only contain display_name
	updateBookVal.AddRule(FieldMaskField("update_mask").OnlyContains([]string{"display_name"}))
	// fields specified in update_mask should be valid according to the book validator
	updateBookVal.AddSubMessageValidator("book", bookVal, &validator.SubMsgOptions{OnlyValidateFieldsSpecifiedIn: "update_mask"})
}

func ExampleValidate() {
	// setup example request interface{}
	testBook := &pbOpen.Book{
		DisplayName:     "The Book",
		Type:            pbOpen.Type_ANONYMOUS,
		PublicationDate: &date.Date{Year: 2021, Month: 1, Day: 1},
	}
	updateBook := &pbOpen.UpdateBookRequest{
		Book:       testBook,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}},
	}
	request := updateBook.ProtoReflect().Interface()

	// normally the following would be called in the server interceptor of your grpc server
	err, found := validator.Validate(request)
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
