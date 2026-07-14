package evals

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"
)

// ServerStreamResult is what CallServerStream returns: drained messages,
// transport error (if any), and timing captured uniformly by the framework.
type ServerStreamResult[T any] struct {
	Messages         []T
	Err              error
	TTFB             time.Duration
	TotalDuration    time.Duration
	MessageIntervals []time.Duration
}

// CallServerStream opens a server-streaming RPC, drains messages until EOF or
// error, and captures timing. TTFB is the elapsed time from call start
// (including openFn) through the first successful Recv; it is 0 when no
// message is received — guard with len(Messages) > 0 before asserting on TTFB.
//
// MessageIntervals[i] is the elapsed time between Messages[i] and Messages[i+1].
// Length is max(0, len(Messages)-1).
//
// If Recv returns a nil message with nil error, Err is set and partial
// messages received so far are preserved.
//
// Do not use on watch/subscribe RPCs that never send EOF — the call blocks
// until context cancellation. Use a context deadline to bound execution.
func CallServerStream[T any](
	ctx context.Context,
	openFn func(context.Context) (grpc.ServerStreamingClient[T], error),
) ServerStreamResult[T] {
	start := time.Now()
	stream, err := openFn(ctx)
	if err != nil {
		return ServerStreamResult[T]{
			Err:           err,
			TotalDuration: time.Since(start),
		}
	}
	if stream == nil {
		return ServerStreamResult[T]{
			Err:           ErrNilStream{},
			TotalDuration: time.Since(start),
		}
	}

	var (
		messages  []T
		intervals []time.Duration
		ttfb      time.Duration
		lastRecv  time.Time
		gotFirst  bool
	)

	for {
		msg, err := stream.Recv()
		now := time.Now()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ServerStreamResult[T]{
				Messages:         messages,
				Err:              err,
				TTFB:             ttfb,
				TotalDuration:    time.Since(start),
				MessageIntervals: intervals,
			}
		}
		if msg == nil {
			return ServerStreamResult[T]{
				Messages:         messages,
				Err:              ErrNilStreamMessage{},
				TTFB:             ttfb,
				TotalDuration:    time.Since(start),
				MessageIntervals: intervals,
			}
		}
		if !gotFirst {
			ttfb = now.Sub(start)
			gotFirst = true
		} else {
			intervals = append(intervals, now.Sub(lastRecv))
		}
		lastRecv = now
		messages = append(messages, *msg)
	}

	return ServerStreamResult[T]{
		Messages:         messages,
		TTFB:             ttfb,
		TotalDuration:    time.Since(start),
		MessageIntervals: intervals,
	}
}
