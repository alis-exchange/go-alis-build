package spanneradapter

import (
	"reflect"
	"testing"

	"cloud.google.com/go/spanner"
)

func TestScanRow(t *testing.T) {
	s := Scanner[string]{Spec: StringKeySpec("key"), Codec: StringCodec(), ResourceColumn: "Res"}
	row, err := spanner.NewRow([]string{"key", "Res"}, []any{"k1", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ScanRow(row)
	if err != nil {
		t.Fatal(err)
	}
	if got.Resource != "hello" {
		t.Fatal(got.Resource)
	}
	if !reflect.DeepEqual(got.Key.KeyValues(), []any{"k1"}) {
		t.Fatal(got.Key)
	}
	if got.Policy != nil {
		t.Fatal("no policy column → nil policy")
	}
}

func TestScannerColumns(t *testing.T) {
	s := Scanner[string]{Spec: StringKeySpec("key"), Codec: StringCodec(), ResourceColumn: "Res", PolicyColumn: "Policy"}
	if !reflect.DeepEqual(s.Columns(), []string{"key", "Res", "Policy"}) {
		t.Fatal(s.Columns())
	}
}
