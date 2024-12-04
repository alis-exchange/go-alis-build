package maps

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestOrderedMap(t *testing.T) {
	type args[K comparable, V any] struct {
		orderedMap *OrderedMap[K, V]
		len        int
	}
	type testCase[K comparable, V any] struct {
		name string
		args args[K, V]
		fn   func(*OrderedMap[K, V], int) bool
		want bool
	}
	tests := []testCase[int, string]{
		{
			name: "Simple Sets",
			args: args[int, string]{
				len:        100,
				orderedMap: NewOrderedMap[int, string](),
			},
			fn: func(om *OrderedMap[int, string], len int) bool {
				for i := range len {
					om.Set(i, fmt.Sprintf("test-%d", i))
				}

				return true
			},
			want: true,
		},
		{
			name: "Concurrent Sets",
			args: args[int, string]{
				len:        100,
				orderedMap: NewOrderedMap[int, string](),
			},
			fn: func(om *OrderedMap[int, string], len int) bool {
				wg := sync.WaitGroup{}

				set := func(i int) {
					defer wg.Done()
					om.Set(i, fmt.Sprintf("test-%d", i))
				}

				for i := range len {
					wg.Add(1)
					go set(i)
				}

				wg.Wait()

				return true
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.args.orderedMap, tt.args.len); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OrderedMap() = %v, want %v", got, tt.want)
			}
			if tt.args.orderedMap.Len() != tt.args.len {
				t.Errorf("OrderedMap.Len() = %v, want %v", tt.args.orderedMap.Len(), tt.args.len)
			}
			tt.args.orderedMap.Range(func(idx int, key int, value string) bool {
				if value != fmt.Sprintf("test-%d", key) {
					t.Errorf("OrderedMap.Range() = %v, want %v", value, fmt.Sprintf("test-%d", key))
					return false
				}
				return true
			})
			if v, ok := tt.args.orderedMap.Get(1); !ok || v != "test-1" {
				t.Errorf("OrderedMap.Get() = %v, want %v", v, "test-1")
			}
			if len(tt.args.orderedMap.Keys()) != tt.args.len {
				t.Errorf("OrderedMap.Keys() = %v, want %v", len(tt.args.orderedMap.Keys()), tt.args.len)
			}
			if len(tt.args.orderedMap.Values()) != tt.args.len {
				t.Errorf("OrderedMap.Values() = %v, want %v", len(tt.args.orderedMap.Values()), tt.args.len)
			}
		})
	}
}
