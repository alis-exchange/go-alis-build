package validator

import (
	"testing"

	pbOpen "open.alis.services/protobuf/alis/open/validation/v1"
)

func Test_asdf(t *testing.T) {
	err, found := Validate(&pbOpen.Book{})
	if err != nil {
		t.Error(err)
	}
	t.Logf("found: %v", found)
}
