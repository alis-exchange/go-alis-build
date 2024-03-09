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

// Validation is used to facilitate using the pre-defined standard set of
// validations included in this package (RequiredFields, RegexFields, etc.), as
// well as providing the interface required for custom validations.
type Validation interface {
	Do(data proto.Message) []Violation
}

// Violation is a single violation result of a particular validation.
type Violation struct {
	FieldPath string
	Message   string
}

// New instantiates a new Validator object, to which Validations would be added
// to and used.
func New(data proto.Message) *Validator {
	return &Validator{
		Data: data,
	}
}

// Execute evaluates the data against the current set of Validations.  If any
// of the Validations fail, this method will generate the relevant Violation entries
func (v *Validator) Execute() *Validator {
	for _, validation := range v.Validations {
		violations := validation.Do(v.Data)
		v.Violations = append(v.Violations, violations...)
	}
	return v
}

// AddValidation adds a single validation to the Validator object
func (v *Validator) AddValidation(Validation Validation) *Validator {
	v.Validations = append(v.Validations, Validation)
	return v
}

// ToRpcStatus generates a single invalid argument error inline with the
// status codes as defined by the `google.rpc.Status` message. This is a
// simple protocol-independent error model, which allows us to offer a consistent
// experience across different APIs, different API protocols (such as gRPC or HTTP),
// and different error contexts (such as asynchronous, batch, or workflow errors).
func (v *Validator) ToRpcStatus() error {
	// Excutes all the validations
	// If any violations
	if len(v.Violations) > 0 {
		// Construct the final concatenate message
		// TODO: construct
		errorMessage := ""
		return status.Errorf(codes.InvalidArgument, errorMessage)

	} else {
		return nil
	}
}
