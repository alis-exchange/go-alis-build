package jwt_test

import (
	"testing"

	"go.alis.build/iam/internal/jwt"
)

func TestParsePayload(t *testing.T) {
	idToken := "insert your access token here to test"
	payload, err := jwt.ParsePayload(idToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", payload)
}
