package sets

import (
	"reflect"
	"testing"
)

func TestSet(t *testing.T) {
	type args[V comparable] struct {
		set *Set[V]
		len int
	}
	type testCase[V comparable] struct {
		name string
		args args[V]
		fn   func(*Set[V], int) int
		want int
	}
	tests := []testCase[int]{
		{
			name: "Simple Sets",
			args: args[int]{
				len: 100,
				set: NewSet[int](101, 102, 103),
			},
			fn: func(s *Set[int], length int) int {
				for i := range length {
					s.Add(i)
				}

				return s.Len()
			},
			want: 103,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.args.set, tt.args.len); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OrderedMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
