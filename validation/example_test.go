package validation_test

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.alis.build/validation"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ExampleNewValidator() {
	req := validation.User{
		Name:       "John",
		Age:        25,
		UpdateTime: timestamppb.Now(),
		Status:     validation.User_ACTIVE,
	}

	// setup validation rules
	v := validation.NewValidator()
	v.String("name", req.GetName()).IsPopulated()
	v.Int32("age", req.GetAge()).Gt(18)
	v.If(v.Enum("status", req.GetStatus()).Is(validation.User_ACTIVE)).Then(
		v.Timestamp("update_time", req.GetUpdateTime()).NotInFuture(),
	)
	v.Or(
		v.String("email", req.GetEmail()).IsEmail().EndsWith(".com"),
		v.String("website", req.GetWebsite()).IsDomain().EndsWith(".com"),
	)

	// validate
	err := v.Validate()
	if err != nil {
		// normally return status.Errorf(codes.InvalidArgument, "%s", err.Error())
		fmt.Println(err)
	}
}

func ExampleValidator_Custom() {
	req := validation.User{
		Name:       "John",
		Age:        25,
		UpdateTime: timestamppb.Now(),
		Status:     validation.User_ACTIVE,
	}

	// setup validation rules
	v := validation.NewValidator()
	v.Custom("name must be populated", req.GetName() != "")
	v.Custom("age must be greater than 18", req.GetAge() > 18)
	v.CustomEvaluated("if status is ACTIVE, update time must not be in the future", func() bool {
		if req.GetStatus() == validation.User_ACTIVE {
			return req.GetUpdateTime().AsTime().Before(time.Now())
		}
		return true
	})
	v.CustomEvaluated("either email must be a valid email and end with .com or website must be a valid domain and end with .com", func() bool {
		emailRgx := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		domainRgx := `^([a-zA-Z0-9]+\.)*[a-zA-Z0-9]+\.[a-zA-Z]{2,}$`
		emailMatch, _ := regexp.MatchString(emailRgx, req.GetEmail())
		domainMatch, _ := regexp.MatchString(domainRgx, req.GetWebsite())
		emailEndsInCom := strings.HasSuffix(req.GetEmail(), ".com")
		domainEndsInCom := strings.HasSuffix(req.GetWebsite(), ".com")
		return (emailMatch && emailEndsInCom) || (domainMatch && domainEndsInCom)
	})

	// validate
	err := v.Validate()
	if err != nil {
		// normally return status.Errorf(codes.InvalidArgument, "%s", err.Error())
		fmt.Println(err)
	}
}
