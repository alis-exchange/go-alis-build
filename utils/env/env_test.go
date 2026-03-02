package env

import (
	"fmt"
	"os"
	"testing"
)

func TestMustGet_Set(t *testing.T) {
	expected := "test_value"
	os.Setenv("TEST_ENV_VAR", expected)
	defer os.Unsetenv("TEST_ENV_VAR")

	actual := MustGet("TEST_ENV_VAR")
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestMustGet_NotSet(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	os.Unsetenv("TEST_ENV_VAR_MISSING") // Ensure it's not set
	MustGet("TEST_ENV_VAR_MISSING")
}

func TestMustExist_AllSet(t *testing.T) {
	os.Setenv("TEST_EXIST_1", "1")
	os.Setenv("TEST_EXIST_2", "2")
	defer os.Unsetenv("TEST_EXIST_1")
	defer os.Unsetenv("TEST_EXIST_2")

	// Should not panic
	MustExist("TEST_EXIST_1", "TEST_EXIST_2")
}

func TestMustExist_Missing(t *testing.T) {
	os.Setenv("TEST_EXIST_1", "1")
	defer os.Unsetenv("TEST_EXIST_1")
	os.Unsetenv("TEST_MISSING_1")
	os.Unsetenv("TEST_MISSING_2")

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("The code did not panic")
		}
		expected := "missing required environment variables: [TEST_MISSING_1 TEST_MISSING_2]"
		if fmt.Sprint(r) != expected {
			t.Errorf("Expected panic message %q, got %q", expected, r)
		}
	}()

	MustExist("TEST_EXIST_1", "TEST_MISSING_1", "TEST_MISSING_2")
}