package validation_test

import (
	"testing"
	"time"

	"go.alis.build/validation"
	"google.golang.org/protobuf/types/known/timestamppb"
	pbIam "open.alis.services/protobuf/alis/open/iam/v1"
)

func Test_ValidateBasic(t *testing.T) {
	v := validation.NewValidator()
	v.String("name", "ohn").IsPopulated()
	err := v.Validate()
	if err != nil {
		t.Error(err)
	}
}

func Test_ValidateOr(t *testing.T) {
	v := validation.NewValidator()
	v.Or(v.String("name", "").IsPopulated(), v.String("email", "d").IsPopulated())
	err := v.Validate()
	if err != nil {
		t.Error(err)
	}
}

func Test_ValidateIf(t *testing.T) {
	v := validation.NewValidator()
	v.If(v.Int32("age", 17).Gt(18)).Then(
		v.String("name", "").IsPopulated(),
	)
	err := v.Validate()
	if err != nil {
		t.Error(err)
	}
}

func Test_ValidateProto(t *testing.T) {
	v := validation.NewValidator()
	user := pbIam.User{
		IdentityProvider: pbIam.IdentityProvider_LINKEDIN,
		CreateTime:       timestamppb.Now(),
		UpdateTime:       timestamppb.New(time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC)),
		// GoogleIdentity:   &pbIam.User_GoogleIdentity{},
	}
	// v.Timestamp("create_time", user.GetCreateTime()).IsPopulated().BeforeOrEq(user.GetUpdateTime(), validation.ReferencedTime("update_time", true))
	// v.Timestamp("create_time", user.GetCreateTime()).NotInFuture().InMonth(11).OnDayOfMonth(27)
	// v.Duration("dur", durationpb.New(5*time.Minute)).LongerThan(durationpb.New(6 * time.Minute))
	// v.Enum("identity_provider", user.GetIdentityProvider()).IsOneof(pbIam.IdentityProvider_EMAIL, pbIam.IdentityProvider_GOOGLE)
	// v.String("name", user.Name).StartsWithOneof("a", "b")
	v.StringList("roles", []string{"roles/asdf", "roles/asdf"}).EachUnique()
	v.Int32List("ages", []int32{1, 2, 1}).IsAscending().LengthGt(2)
	v.If(v.Bool("verified_email", user.VerifiedEmail).True()).Then(
		v.String("email", user.Email).IsEmail(),
	)
	// v.MessageIsPopulated("google_identity", user.GetGoogleIdentity() != nil)
	err := v.Validate()
	if err != nil {
		t.Error(err)
	}
}
