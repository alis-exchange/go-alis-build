package filtering

import (
	"testing"

	"cloud.google.com/go/spanner"
)

func TestNewFilter(t *testing.T) {

	identifiers := []Identifier{
		Timestamp("create_time"),
		Duration("expire_after"),
	}
	filter, err := NewFilter(identifiers...)
	if err != nil {
		t.Errorf("NewFilter() error = %v", err)
		return
	}

	// filter.DeclareIdentifier(Timestamp("create_time"))

	type args struct {
		filter string
	}
	tests := []struct {
		name    string
		args    args
		want    *spanner.Statement
		wantErr bool
	}{
		{
			name: "TestNewFilter",
			args: args{
				filter: "name = 'Alice' AND create_time > timestamp('2021-01-01T00:00:00Z')",
			},
			want:    &spanner.Statement{},
			wantErr: false,
		},
		{
			name: "TestFilter_Prefix",
			args: args{
				filter: "age > 18 AND prefix(name, 'Alice') AND suffix(name, 'Alice')",
			},
			want:    &spanner.Statement{},
			wantErr: false,
		},
		{
			name: "TestFilter_In",
			args: args{
				filter: "name IN ['Alice', 'Bob']",
			},
			want:    &spanner.Statement{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filter.Parse(tt.args.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("filter.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Logf("got SQL: %s", got.SQL)
			t.Logf("got Params: %+v", got.Params)
		})
	}
}
