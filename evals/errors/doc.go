// Package errors bridges evals-typed errors to gRPC statuses at the RPC
// boundary.
//
// Typed errors in the evals module implement [EvalError], which adds a
// `GRPCStatus() *status.Status` method to the standard `error` interface. That
// extra method lets [ToGRPC] and [ToGRPCf] preserve the intended code instead
// of collapsing everything to Unknown.
//
// # Usage in RPC handlers
//
//	if err := validateRequest(req); err != nil {
//	    return nil, evalerrors.ToGRPCf("case_ids", err)
//	}
//
// [ToGRPCf] prepends a field name to the message, which is the idiom for
// validation errors. [ToGRPC] does not.
//
// Errors that do not implement EvalError fall back to InvalidArgument
// wrapping — deliberately, since evals RPC handlers rarely surface
// server-internal failures directly. Wrap those explicitly with
// `status.Errorf(codes.Internal, ...)` if you want.
package errors
