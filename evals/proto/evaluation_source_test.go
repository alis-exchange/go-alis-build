package proto

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEvaluationProtoSource_validationContract(t *testing.T) {
	t.Parallel()

	file := loadEvaluationSourceFile(t)

	validation := file.Messages().ByName("Validation")
	if validation == nil {
		t.Fatal("Validation message not defined in evaluation.proto source")
	}
	assertField(t, validation, 1, "id", protoreflect.StringKind)
	assertField(t, validation, 2, "status", protoreflect.EnumKind)
	assertField(t, validation, 3, "message", protoreflect.StringKind)
	status := validation.Fields().ByNumber(2)
	if got, want := status.Enum().FullName(), protoreflect.FullName("alis.evals.v1.Status"); got != want {
		t.Fatalf("Validation.status enum = %q, want %q", got, want)
	}

	tt := []struct {
		name       string
		casePath   []string
		wantNumber protoreflect.FieldNumber
	}{
		{name: "agent_eval", casePath: []string{"AgentEvalResults", "Case"}, wantNumber: 6},
		{name: "load_test", casePath: []string{"LoadTestResults", "Case"}, wantNumber: 9},
		{name: "infra_observation", casePath: []string{"InfraObservationResults", "Case"}, wantNumber: 9},
	}
	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			caseDesc := lookupSourceMessage(t, file, tc.casePath...)
			fd := caseDesc.Fields().ByName("validations")
			if fd == nil {
				t.Fatalf("%s.validations not defined in evaluation.proto", joinMessagePath(tc.casePath))
			}
			if fd.Number() != tc.wantNumber {
				t.Fatalf("validations number = %d, want %d", fd.Number(), tc.wantNumber)
			}
			if !fd.IsList() || fd.Message().FullName() != validation.FullName() {
				t.Fatalf("validations field shape mismatch")
			}
		})
	}

	intCase := lookupSourceMessage(t, file, "IntegrationTestResults", "Case")
	if intCase.Fields().ByName("validations") != nil {
		t.Fatal("IntegrationTestResults.Case must not define validations")
	}
}

func loadEvaluationSourceFile(t *testing.T) protoreflect.FileDescriptor {
	t.Helper()

	root := commonProtosRoot(t)
	protoPath := filepath.Join(root, "alis", "evals", "v1", "evaluation.proto")
	if _, err := os.Stat(protoPath); err != nil {
		t.Fatalf("evaluation.proto not found at %s: %v", protoPath, err)
	}

	descriptorPath := filepath.Join(t.TempDir(), "evaluation.pb")
	cmd := exec.Command(
		"protoc",
		"-I", root,
		"--descriptor_set_out="+descriptorPath,
		"--include_imports",
		"alis/evals/v1/evaluation.proto",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatalf("read descriptor set: %v", err)
	}

	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(raw, &set); err != nil {
		t.Fatalf("unmarshal descriptor set: %v", err)
	}
	files, err := protodesc.NewFiles(&set)
	if err != nil {
		t.Fatalf("protodesc.NewFiles: %v", err)
	}
	fd, err := files.FindFileByPath("alis/evals/v1/evaluation.proto")
	if err != nil {
		t.Fatalf("FindFileByPath: %v", err)
	}
	return fd
}

func lookupSourceMessage(t *testing.T, file protoreflect.FileDescriptor, path ...string) protoreflect.MessageDescriptor {
	t.Helper()
	desc := file.Messages().ByName(protoreflect.Name(path[0]))
	for _, segment := range path[1:] {
		if desc == nil {
			t.Fatalf("message %q not found in evaluation.proto source", joinMessagePath(path))
		}
		desc = desc.Messages().ByName(protoreflect.Name(segment))
	}
	if desc == nil {
		t.Fatalf("message %q not found in evaluation.proto source", joinMessagePath(path))
	}
	return desc
}

func commonProtosRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("COMMON_PROTOS_ROOT"); root != "" {
		return root
	}
	t.Skip("source contract test requires COMMON_PROTOS_ROOT; installed descriptor tests run by default")
	return ""
}
