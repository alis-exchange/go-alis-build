package evals

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"
)

// Result is what Call returns: the typed response, transport error (if any),
// and wall-clock latency. Assertions are the author's responsibility.
type Result[T any] struct {
	Resp    T
	Err     error
	Latency time.Duration
}

// Call invokes fn and captures the response, error, and wall-clock latency.
// The type parameter is inferred from fn, so authors never write it explicitly.
//
//	r := evals.Call(ctx, func(ctx context.Context) (*filespb.File, error) {
//	    return clients.Files.GetFile(ctx, req)
//	})
//	v.Custom("grpc.status_ok", r.Err == nil)
//	v.Custom("latency.within_budget", r.Latency <= budget)
func Call[T any](ctx context.Context, fn func(context.Context) (T, error)) Result[T] {
	start := time.Now()
	resp, err := fn(ctx)
	return Result[T]{Resp: resp, Err: err, Latency: time.Since(start)}
}

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

// ClientStreamResult is what CallClientStream returns: the typed response,
// transport error (if any), and send/response timing captured by the framework.
//
// SendDuration includes stream open through the last successful Send (or through
// the first send error, or CloseAndRecv entry when no sends occurred). It is
// not send-only wall time — openFn cost is included, consistent with server
// streaming TTFB.
//
// ResponseLatency spans CloseAndRecv entry through return. It is 0 when
// CloseAndRecv was never reached.
//
// TotalDuration is wall clock from call start through sendFn return.
// TotalDuration may exceed SendDuration + ResponseLatency because the gap
// between the last Send return and CloseAndRecv entry is author overhead.
type ClientStreamResult[Resp any] struct {
	Resp            Resp
	Err             error
	SendDuration    time.Duration
	ResponseLatency time.Duration
	TotalDuration   time.Duration
	MessagesSent    int
}

// clientStreamTiming accumulates wall-clock checkpoints for client-stream send
// and CloseAndRecv phases; CallClientStream reads it to populate ClientStreamResult.
type clientStreamTiming struct {
	start             time.Time
	messagesSent      int
	lastSendEnd       time.Time
	sendFailedAt      time.Time
	closeAndRecvStart time.Time
	closeAndRecvEnd   time.Time
	closeInvoked      bool
	sendFailed        bool
	hadSuccessfulSend bool
}

// instrumentedClientStream wraps a gRPC client stream to record send and
// CloseAndRecv timing for load SLO aggregation.
type instrumentedClientStream[Req, Resp any] struct {
	grpc.ClientStreamingClient[Req, Resp]
	timing *clientStreamTiming
}

// Send records send outcome and updates timing counters before delegating.
func (w *instrumentedClientStream[Req, Resp]) Send(req *Req) error {
	err := w.ClientStreamingClient.Send(req)
	now := time.Now()
	if err != nil {
		w.timing.sendFailed = true
		w.timing.sendFailedAt = now
		return err
	}
	w.timing.messagesSent++
	w.timing.hadSuccessfulSend = true
	w.timing.lastSendEnd = now
	return nil
}

// CloseAndRecv records CloseAndRecv wall times before delegating.
func (w *instrumentedClientStream[Req, Resp]) CloseAndRecv() (*Resp, error) {
	w.timing.closeInvoked = true
	w.timing.closeAndRecvStart = time.Now()
	resp, err := w.ClientStreamingClient.CloseAndRecv()
	w.timing.closeAndRecvEnd = time.Now()
	return resp, err
}

// sendDuration derives TTFB-style send duration from recorded timing events.
func sendDuration(t *clientStreamTiming) time.Duration {
	switch {
	case t.hadSuccessfulSend:
		return t.lastSendEnd.Sub(t.start)
	case t.sendFailed:
		return t.sendFailedAt.Sub(t.start)
	case t.closeInvoked:
		return t.closeAndRecvStart.Sub(t.start)
	default:
		return time.Since(t.start)
	}
}

// responseLatency returns CloseAndRecv wall time; zero when never invoked.
func responseLatency(t *clientStreamTiming) time.Duration {
	if !t.closeInvoked {
		return 0
	}
	return t.closeAndRecvEnd.Sub(t.closeAndRecvStart)
}

// CallClientStream opens a client-streaming RPC, invokes sendFn with an
// instrumented stream that counts sends and records send vs response timing,
// and returns the aggregated result. Assertions are the author's responsibility.
func CallClientStream[Req, Resp any](
	ctx context.Context,
	openFn func(context.Context) (grpc.ClientStreamingClient[Req, Resp], error),
	sendFn func(grpc.ClientStreamingClient[Req, Resp]) (Resp, error),
) ClientStreamResult[Resp] {
	start := time.Now()
	timing := &clientStreamTiming{start: start}

	stream, err := openFn(ctx)
	if err != nil {
		return ClientStreamResult[Resp]{
			Err:           err,
			TotalDuration: time.Since(start),
		}
	}
	if stream == nil {
		return ClientStreamResult[Resp]{
			Err:           ErrNilStream{},
			TotalDuration: time.Since(start),
		}
	}

	wrapped := &instrumentedClientStream[Req, Resp]{
		ClientStreamingClient: stream,
		timing:                timing,
	}
	resp, err := sendFn(wrapped)

	return ClientStreamResult[Resp]{
		Resp:            resp,
		Err:             err,
		SendDuration:    sendDuration(timing),
		ResponseLatency: responseLatency(timing),
		TotalDuration:   time.Since(start),
		MessagesSent:    timing.messagesSent,
	}
}
