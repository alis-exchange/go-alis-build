package proto

import (
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestValidation_messageShape(t *testing.T) {
	t.Parallel()

	desc := fileDescriptor(t).Messages().ByName("Validation")
	if desc == nil {
		t.Fatal("Validation message not defined in alis.evals.v1")
	}
	assertField(t, desc, 1, "id", protoreflect.StringKind)
	assertField(t, desc, 2, "status", protoreflect.EnumKind)
	assertField(t, desc, 3, "message", protoreflect.StringKind)
	status := desc.Fields().ByNumber(2)
	if got, want := status.Enum().FullName(), protoreflect.FullName("alis.evals.v1.Status"); got != want {
		t.Fatalf("Validation.status enum = %q, want %q", got, want)
	}
	if desc.Fields().Len() != 3 {
		t.Fatalf("Validation field count = %d, want 3", desc.Fields().Len())
	}
}

func TestSpecializedCases_validationsFieldNumbers(t *testing.T) {
	t.Parallel()

	validation := fileDescriptor(t).Messages().ByName("Validation")
	if validation == nil {
		t.Fatal("Validation message not defined")
	}

	tt := []struct {
		name       string
		casePath   []string
		wantNumber protoreflect.FieldNumber
	}{
		{
			name:       "agent_eval",
			casePath:   []string{"AgentEvalResults", "Case"},
			wantNumber: 6,
		},
		{
			name:       "load_test",
			casePath:   []string{"LoadTestResults", "Case"},
			wantNumber: 9,
		},
		{
			name:       "infra_observation",
			casePath:   []string{"InfraObservationResults", "Case"},
			wantNumber: 9,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			caseDesc := lookupMessage(t, tc.casePath...)
			fd := caseDesc.Fields().ByName("validations")
			if fd == nil {
				t.Fatalf("%s.validations not defined", joinMessagePath(tc.casePath))
			}
			if fd.Number() != tc.wantNumber {
				t.Fatalf("validations number = %d, want %d", fd.Number(), tc.wantNumber)
			}
			if !fd.IsList() {
				t.Fatal("validations must be repeated")
			}
			if fd.Message().FullName() != validation.FullName() {
				t.Fatalf("validations type = %v, want %v", fd.Message().FullName(), validation.FullName())
			}
		})
	}
}

func TestIntegrationCase_unchanged(t *testing.T) {
	t.Parallel()

	caseDesc := lookupMessage(t, "IntegrationTestResults", "Case")
	if caseDesc.Fields().ByName("validations") != nil {
		t.Fatal("integration TestResults.Case must not define validations")
	}

	want := []struct {
		number protoreflect.FieldNumber
		name   protoreflect.Name
		kind   protoreflect.Kind
	}{
		{1, "id", protoreflect.StringKind},
		{2, "status", protoreflect.EnumKind},
		{3, "checks", protoreflect.MessageKind},
		{4, "duration", protoreflect.MessageKind},
	}
	if caseDesc.Fields().Len() != len(want) {
		t.Fatalf("integration case field count = %d, want %d", caseDesc.Fields().Len(), len(want))
	}
	for _, field := range want {
		fd := caseDesc.Fields().ByNumber(field.number)
		if fd == nil {
			t.Fatalf("field %d missing", field.number)
		}
		if fd.Name() != field.name {
			t.Fatalf("field %d name = %q, want %q", field.number, fd.Name(), field.name)
		}
		if fd.Kind() != field.kind {
			t.Fatalf("field %d kind = %v, want %v", field.number, fd.Kind(), field.kind)
		}
	}
}

func TestExistingRunEnvelope_unchanged(t *testing.T) {
	t.Parallel()

	run := (&evalspb.Run{}).ProtoReflect().Descriptor()
	want := []struct {
		number protoreflect.FieldNumber
		name   protoreflect.Name
	}{
		{2, "name"},
		{3, "batch_id"},
		{4, "type"},
		{5, "status"},
		{6, "integration_test"},
		{7, "load_test"},
		{8, "agent_eval"},
		{9, "infra_observation"},
		{21, "start_time"},
		{22, "end_time"},
		{23, "operation"},
		{24, "error"},
		{25, "create_time"},
		{26, "google_project_id"},
	}
	for _, field := range want {
		fd := run.Fields().ByNumber(field.number)
		if fd == nil {
			t.Fatalf("Run.%s (field %d) missing", field.name, field.number)
		}
		if fd.Name() != field.name {
			t.Fatalf("Run field %d name = %q, want %q", field.number, fd.Name(), field.name)
		}
	}
}

func fileDescriptor(t *testing.T) protoreflect.FileDescriptor {
	t.Helper()
	return (&evalspb.Run{}).ProtoReflect().Descriptor().ParentFile()
}

func lookupMessage(t *testing.T, path ...string) protoreflect.MessageDescriptor {
	t.Helper()
	if len(path) == 0 {
		t.Fatal("empty message path")
	}
	desc := fileDescriptor(t).Messages().ByName(protoreflect.Name(path[0]))
	for _, segment := range path[1:] {
		if desc == nil {
			t.Fatalf("message %q not found", joinMessagePath(path))
		}
		desc = desc.Messages().ByName(protoreflect.Name(segment))
	}
	if desc == nil {
		t.Fatalf("message %q not found", joinMessagePath(path))
	}
	return desc
}

func assertField(t *testing.T, desc protoreflect.MessageDescriptor, number protoreflect.FieldNumber, name protoreflect.Name, kind protoreflect.Kind) {
	t.Helper()
	fd := desc.Fields().ByNumber(number)
	if fd == nil {
		t.Fatalf("%s: field %d missing", desc.Name(), number)
	}
	if fd.Name() != name {
		t.Fatalf("%s field %d name = %q, want %q", desc.Name(), number, fd.Name(), name)
	}
	if fd.Kind() != kind {
		t.Fatalf("%s.%s kind = %v, want %v", desc.Name(), name, fd.Kind(), kind)
	}
}

func joinMessagePath(path []string) string {
	out := "alis.evals.v1"
	for _, segment := range path {
		out += "." + segment
	}
	return out
}
