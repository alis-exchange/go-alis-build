package jwt_test

import (
	"testing"

	"github.com/alis-exchange/iam/internal/jwt"
)

func TestParsePayload(t *testing.T) {
	idToken := "insert your access token here to test"
	payload, err := jwt.ParsePayload(idToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", payload)
}
