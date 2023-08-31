package filtering

import (
	"cloud.google.com/go/firestore"
	"go.einride.tech/aip/filtering"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ParseFilter accepts a filtering.Request, which is an interface representing List method Request messages
// that have a GetFilter method which returns the filter specified in the request.
func ParseFilter(messageType proto.Message, filterString string, query *firestore.Query) (*firestore.Query, error) {
	// Use protoreflect to register each field and its type as a declaration for filtering
	declarationOpts := []filtering.DeclarationOption{
		filtering.DeclareStandardFunctions(),
	}

	messageReflect := messageType.ProtoReflect()
	messageReflect.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Kind() {
		case protoreflect.BoolKind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeBool)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeBool))
			}
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
			protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeInt)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeInt))
			}
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
			protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeInt)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeInt))
			}
		case protoreflect.FloatKind, protoreflect.DoubleKind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeFloat)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeFloat))
			}
		case protoreflect.StringKind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeString)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeString))
			}
		case protoreflect.BytesKind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeString)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeString))
			}
		// TODO: how do we handle enums with the filtering.EnumType instead
		case protoreflect.EnumKind:
			if fd.Cardinality() == protoreflect.Repeated {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeList(filtering.TypeInt)))
			} else {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeString))
			}
		// Duration or timestamp
		case protoreflect.MessageKind:
			if fd.Message().FullName() == "google.protobuf.Timestamp" {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeTimestamp))
			}
			if fd.Message().FullName() == "google.protobuf.Duration" {
				declarationOpts = append(declarationOpts, filtering.DeclareIdent(string(fd.Name()), filtering.TypeDuration))
			}
		}
		return true
	})

	declarations, err := filtering.NewDeclarations(declarationOpts...)
	if err != nil {
		return nil, err
	}

	// Instantiate a new go.einride.tech/aip/filtering.Filter for use by the transpiler
	filter := filtering.Filter{}
	if filterString == "" {
		return TranspileFilterToQuery(filter, query)
	}
	var parser filtering.Parser
	parser.Init(filterString)
	parsedExpr, err := parser.Parse()
	if err != nil {
		return nil, err
	}
	var checker filtering.Checker
	checker.Init(parsedExpr.Expr, parsedExpr.SourceInfo, declarations)
	checkedExpr, err := checker.Check()
	if err != nil {
		return nil, err
	}
	filter = filtering.Filter{
		CheckedExpr: checkedExpr,
	}

	// Validate portfolio against the filter
	return TranspileFilterToQuery(filter, query)
}
