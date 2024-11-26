package validation_test

import (
	"context"
	"testing"

	"go.alis.build/validation"
)

type TestStruct struct {
	Name    string
	Surname string
	Age     int
	Email   string
	Valid   bool
	// CreateTime *timestamppb.Timestamp
	// UpdateTime *timestamppb.Timestamp
}

func (t *TestStruct) GetName() string {
	return t.Name
}

func (t *TestStruct) GetSurname() string {
	return t.Surname
}

func (t *TestStruct) GetAge() int {
	return t.Age
}

func (t *TestStruct) GetEmail() string {
	return t.Email
}

func (t *TestStruct) GetValid() bool {
	return t.Valid
}

// func (t *TestStruct) GetCreateTime() *timestamppb.Timestamp {
// 	return t.CreateTime
// }

// func (t *TestStruct) GetUpdateTime() *timestamppb.Timestamp {
// 	return t.UpdateTime
// }

func Test_Validation(t *testing.T) {
	v := validation.NewValidator(context.Background())
	v.Gt("age", 10, 18)
	err := v.Validate()
	if err != nil {
		t.Error(err)
	}
	// req := &TestStruct{}
	// v := validation.NewValidator(context.Background())

	// // name must be populated
	// {
	// 	// super basic
	// 	v.AddBasicRule("name must be populated", req.GetName() != "")

	// 	// allows for dependencies
	// 	v.AddEvaluatedRule("name must be populated", func() bool { return req.GetName() != "" })

	// 	// standard provided
	// 	v.AddRule(v.StringField("name", req.GetName).Populated())
	// }

	// // name or surname must be populated
	// {
	// 	// super basic
	// 	v.AddBasicRule("name or surname must be populated", req.GetName() != "" || req.GetSurname() != "")

	// 	// allows for dependencies
	// 	v.AddEvaluatedRule("name or surname must be populated", func() bool { return req.GetName() != "" || req.GetSurname() != "" })

	// 	// standard provided
	// 	v.AddRule(v.OR(
	// 		v.SF("name", req.GetName).Populated(), v.SF("surname", req.GetSurname).Populated(),
	// 	))
	// // }

	// err := validation.NewValidator(context.Background())
	// .Required("name", req.GetName()).MatchesRegex("name", req.GetName(), "^[a-zA-Z]+$").InRange("age", req.GetAge(), 18, 100).CustomRule("the sum of all the percentages in column A must sum up to 100",func() error { return errors.New("currently summing up to 104")})

	// // combine "rule: failureReason"
	// v.IsEmail("email", req.GetEmail())

	// v.MatchesRegex("subfield.somesub.name")
	// v.AddRule(v.StringField("name", req.GetName).Populated())
	// err := v.AddEvalutedRule("name must be populated", func() bool { return req.Name != "" }).AddBasicRule("age must be greater than 18", req.Age > 18)
	// if err != nil {
	// 	return err
	// }

	// v.CustomRule()
	// v.Matches()
	// v.MatchesOneof()
	// v.String("name",req.GetName()).Matches(req.Regex,validation.WithValueDescription(""))
	// v.Timestamp("create_time",req.GetCreateTime()).After(req.GetUpdateTime(), validation.WithDescription("update_time")).If
	// v.Enum("status", req.Status).Is("ACTIVE", "INACTIVE")
}
