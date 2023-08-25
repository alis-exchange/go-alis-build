package utils

import (
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
