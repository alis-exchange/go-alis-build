package evals

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

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
	// start is set once at CallClientStream entry.
	start time.Time
	// messagesSent increments after each successful Send.
	messagesSent int
	// lastSendEnd is wall time after the most recent successful Send.
	lastSendEnd time.Time
	// sendFailedAt is wall time when the first Send error occurred.
	sendFailedAt time.Time
	// closeAndRecvStart is set when CloseAndRecv is entered.
	closeAndRecvStart time.Time
	// closeAndRecvEnd is set when CloseAndRecv returns.
	closeAndRecvEnd time.Time
	// closeInvoked is true once CloseAndRecv has been called.
	closeInvoked bool
	// sendFailed is true once any Send returns an error.
	sendFailed bool
	// hadSuccessfulSend is true once at least one Send succeeds.
	hadSuccessfulSend bool
}

// instrumentedClientStream wraps a gRPC client stream to record send and
// CloseAndRecv timing for load SLO aggregation.
type instrumentedClientStream[Req, Resp any] struct {
	grpc.ClientStreamingClient[Req, Resp]
	// timing holds shared checkpoint state for one CallClientStream invocation.
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
// Prefers last successful send end; falls back to first send error or
// CloseAndRecv entry when no sends succeeded.
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
