package ordering

import (
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

			t.Logf("SortOrder = %v", got.SortOrder())
		})
	}
}
