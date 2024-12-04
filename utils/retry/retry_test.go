package retry

import (
	"reflect"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	type args[R interface{}] struct {
		attempts  int
		baseSleep time.Duration
		f         func() (R, error)
	}
	type testCase[R interface{}] struct {
		name    string
		args    args[R]
		want    R
		wantErr bool
	}
	tests := []testCase[any]{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Retry(tt.args.attempts, tt.args.baseSleep, tt.args.f)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Retry() got = %v, want %v", got, tt.want)
			}
		})
	}
}
