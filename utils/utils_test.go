package utils

import (
	"reflect"
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
