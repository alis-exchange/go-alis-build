package validator

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Validator struct {
	Type        string
	Validations []Validation
	IsValid     bool
	Violations  []Violation
	Data        proto.Message
}

type Validation interface {
	Do(data proto.Message) []Violation
}

type Violation struct {
	FieldPath string
	Message   string
}

// New creates a Validator object
func New(data proto.Message) *Validator {
	return &Validator{
		Data: data,
	}
}

// Execute iterates over the
func (v *Validator) Execute() {
	for _, validation := range v.Validations {
		violations := validation.Do(v.Data)
		v.Violations = append(v.Violations, violations...)
	}
}

// AddValidation adds a single validation to the Validator object
func (v *Validator) AddValidation(Validation Validation) {
	v.Validations = append(v.Validations, Validation)
}

func (v *Validator) ValidateWithRpcStatus() error {
	// Excutes all the validations
	v.Execute()

	// If any violations
	if len(v.Violations) > 0 {
		// Construct the final concatenate message
		// TODO: construct
		errorMessage := ""
		return status.Errorf(codes.Internal, errorMessage)

	} else {
		return nil
	}
}
