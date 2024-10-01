package utils

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestContainsString(t *testing.T) {
	type args[T comparable] struct {
		s          []T
		searchTerm T
	}
	type testCase[T comparable] struct {
		name string
		args args[T]
		want bool
	}
	tests := []testCase[string]{
		{
			name: "String",
			args: args[string]{
				s:          []string{"test", "test1"},
				searchTerm: "test",
			},
			want: true,
		},
		{
			name: "String:False",
			args: args[string]{
				s:          []string{"test", "test1"},
				searchTerm: "test3",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.s, tt.args.searchTerm); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsInt(t *testing.T) {
	type args[T comparable] struct {
		s          []T
		searchTerm T
	}
	type testCase[T comparable] struct {
		name string
		args args[T]
		want bool
	}
	tests := []testCase[int]{
		{
			name: "Int",
			args: args[int]{
				s:          []int{12, 31, 43},
				searchTerm: 12,
			},
			want: true,
		},
		{
			name: "Int:False",
			args: args[int]{
				s:          []int{12, 31, 43},
				searchTerm: 1,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.s, tt.args.searchTerm); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransform(t *testing.T) {
	type args[T any, U any] struct {
		arr []T
		fn  func(T) U
	}
	type testCase[T any, U any] struct {
		name string
		args args[T, U]
		want []U
	}
	tests := []testCase[int, int]{
		{
			name: "Int:Double",
			args: args[int, int]{
				arr: []int{1, 2, 3},
				fn:  func(i int) int { return i * 2 },
			},
			want: []int{2, 4, 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Transform(tt.args.arr, tt.args.fn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Transform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFind(t *testing.T) {
	type args[T any] struct {
		arr []T
		fn  func(T) bool
	}
	type testCase[T any] struct {
		name  string
		args  args[T]
		want  T
		want1 int
		want2 bool
	}
	tests := []testCase[string]{
		{
			name: "Simple String True",
			args: args[string]{
				arr: []string{"test", "test1"},
				fn:  func(s string) bool { return s == "test" },
			},
			want:  "test",
			want1: 0,
			want2: true,
		},
		{
			name: "Simple String False",
			args: args[string]{
				arr: []string{"test", "test1"},
				fn:  func(s string) bool { return s == "test3" },
			},
			want:  "",
			want1: -1,
			want2: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := Find(tt.args.arr, tt.args.fn)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Find() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Find() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("Find() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	type args[T any] struct {
		arr []T
		fn  func(T) bool
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want []T
	}
	tests := []testCase[string]{
		{
			name: "Simple String",
			args: args[string]{
				arr: []string{"test", "test1"},
				fn:  func(s string) bool { return s == "test" },
			},
			want: []string{"test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filter(tt.args.arr, tt.args.fn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReduce(t *testing.T) {
	type args[T any, R any] struct {
		arr     []T
		fn      func(R, T) R
		initial R
	}
	type testCase[T any, R any] struct {
		name string
		args args[T, R]
		want R
	}
	tests := []testCase[int, int]{
		{
			name: "Simple Int",
			args: args[int, int]{
				arr:     []int{1, 2, 3},
				fn:      func(r int, i int) int { return r + i },
				initial: 0,
			},
			want: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reduce(tt.args.arr, tt.args.fn, tt.args.initial); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reduce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChunk(t *testing.T) {
	type args[T any] struct {
		arr  []T
		size int
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want [][]T
	}
	tests := []testCase[int]{
		{
			name: "Simple Int",
			args: args[int]{
				arr:  []int{1, 2, 3, 4, 5},
				size: 2,
			},
			want: [][]int{{1, 2}, {3, 4}, {5}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Chunk(tt.args.arr, tt.args.size); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Chunk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	type args[T comparable] struct {
		arr []T
	}
	type testCase[T comparable] struct {
		name string
		args args[T]
		want []T
	}
	tests := []testCase[string]{
		{
			name: "Simple String",
			args: args[string]{
				arr: []string{"test", "test1", "test"},
			},
			want: []string{"test", "test1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Unique(tt.args.arr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unique() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupBy(t *testing.T) {
	type args[T any, K comparable] struct {
		arr []T
		fn  func(T) K
	}
	type testCase[T any, K comparable] struct {
		name string
		args args[T, K]
		want map[K][]T
	}
	tests := []testCase[int, string]{
		{
			name: "Simple Int",
			args: args[int, string]{
				arr: []int{1, 2, 3, 4, 5},
				fn: func(i int) string {
					if i%2 == 0 {
						return "even"
					}
					return "odd"
				},
			},
			want: map[string][]int{"even": {2, 4}, "odd": {1, 3, 5}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupBy(tt.args.arr, tt.args.fn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GroupBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
