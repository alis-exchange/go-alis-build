package evals

import (
	"context"
	"time"

	"go.alis.build/evals/loadgen"
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

type instrumentedClientStream[Req, Resp any] struct {
	grpc.ClientStreamingClient[Req, Resp]
	timing *clientStreamTiming
}

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

func (w *instrumentedClientStream[Req, Resp]) CloseAndRecv() (*Resp, error) {
	w.timing.closeInvoked = true
	w.timing.closeAndRecvStart = time.Now()
	resp, err := w.ClientStreamingClient.CloseAndRecv()
	w.timing.closeAndRecvEnd = time.Now()
	return resp, err
}

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

func responseLatency(t *clientStreamTiming) time.Duration {
	if !t.closeInvoked {
		return 0
	}
	return t.closeAndRecvEnd.Sub(t.closeAndRecvStart)
}

// ClientStreamTargetResult maps a [ClientStreamResult] into a [TargetResult]
// with stream timing populated for load case aggregation. Use as the return
// value from a load [ResultTarget] when exercising client-streaming RPCs;
// pair with [SLOStreamTTFB] and [SLOMessagesPerSec] for stream SLOs.
func ClientStreamTargetResult[Resp any](r ClientStreamResult[Resp]) TargetResult {
	tr := TargetResult{TransportErr: r.Err}
	if r.SendDuration > 0 || r.ResponseLatency > 0 || r.TotalDuration > 0 || r.MessagesSent > 0 {
		tr.Stream = &loadgen.StreamSample{
			SendDuration:    r.SendDuration,
			ResponseLatency: r.ResponseLatency,
			TotalDuration:   r.TotalDuration,
			MessagesSent:    r.MessagesSent,
		}
	}
	return tr
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
