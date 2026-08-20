package ordering

import (
	"reflect"
	"testing"
)

func TestNewOrder(t *testing.T) {
	type args struct {
		order string
	}
	tests := []struct {
		name    string
		args    args
		want    *Order
		wantErr bool
	}{
		{
			name: "TestNewOrder",
			args: args{
				order: "name",
			},
			want:    &Order{},
			wantErr: false,
		},
		{
			name: "TestNewOrder_ValidOrder",
			args: args{
				order: "update_time desc, name",
			},
			want:    &Order{},
			wantErr: false,
		},
		{
			name: "TestNewOrder_InvalidOrder_1",
			args: args{
				order: "update_time desc, name,",
			},
			want:    &Order{},
			wantErr: true,
		},
		{
			name: "TestNewOrder_InvalidOrder_2",
			args: args{
				order: "update_time   desc, name",
			},
			want:    &Order{},
			wantErr: true,
		},
		{
			name: "TestNewOrder_InvalidOrder_3",
			args: args{
				order: "update_time test",
			},
			want:    &Order{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewOrder(tt.args.order, WithDefaultOrder(SortOrderDesc))
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOrder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Verify Columns() works with WithDefaultOrder option
			cols := got.Columns()
			if len(tt.args.order) > 0 && len(cols) == 0 {
				t.Errorf("expected columns for order %q", tt.args.order)
			}
		})
	}
}

func TestColumnsPreservesInputOrder(t *testing.T) {
	o, err := NewOrder("b desc, a asc, c")
	if err != nil {
		t.Fatal(err)
	}
	got := o.Columns()
	want := []ColumnOrder{{"b", true}, {"a", false}, {"c", false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestInvert(t *testing.T) {
	o, _ := NewOrder("a desc, b")
	got := o.Invert().Columns()
	want := []ColumnOrder{{"a", false}, {"b", true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEmptyOrder(t *testing.T) {
	o, _ := NewOrder("")
	if o.Columns() != nil {
		t.Fatal("empty order must return nil columns")
	}
}

func TestNilReceiverAndEmptyInvert(t *testing.T) {
	// Test nil receiver for Columns()
	var nilOrder *Order
	if nilOrder.Columns() != nil {
		t.Fatal("nil receiver Columns() must return nil")
	}

	// Test nil receiver for Invert()
	if nilOrder.Invert() != nil {
		t.Fatal("nil receiver Invert() must return nil")
	}

	// Test Invert() on empty Order produces Order with nil Columns
	emptyOrder, _ := NewOrder("")
	inverted := emptyOrder.Invert()
	if inverted == nil {
		t.Fatal("Invert() on empty Order should not return nil")
	}
	if inverted.Columns() != nil {
		t.Fatal("Invert() on empty Order must have nil Columns()")
	}
}
