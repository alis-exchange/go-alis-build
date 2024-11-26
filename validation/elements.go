package validation

import (
	"context"

	"go.alis.build/alog"
)

// import (
// 	"fmt"
// )

// func Excludes[S ~[]E, E comparable](v *Validator, path string, slice S, values ...E) *Validator {
// 	satisfied := true
// 	for _, s := range slice {
// 		for _, v := range values {
// 			if s == v {
// 				satisfied = false
// 			}
// 		}
// 	}
// 	rule := &Rule{
// 		description: fmt.Sprintf("%s must not contain %s", path, d),
// 	}
// }

func sliceOfAny(path string, value any) []any {
	res := []any{}
	switch value.(type) {
	case []int:
		for _, v := range value.([]int) {
			res = append(res, v)
		}
	default:
		alog.Warnf(context.Background(), "%s is not a primitive slice", path)
	}
}
