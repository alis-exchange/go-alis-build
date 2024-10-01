package validator_test

import (
	"context"
	"testing"

	"go.alis.build/validator"
	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func Test_asdf(t *testing.T) {
	ctx := context.Background()
	err, found := validator.Validate(ctx, &pbOpen.Book{})
	if err != nil {
		t.Error(err)
	}
	t.Logf("found: %v", found)
}
