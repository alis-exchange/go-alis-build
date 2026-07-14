package evals

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeClientStream struct {
	ctx context.Context
}

func (f *fakeClientStream) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeClientStream) Trailer() metadata.MD         { return nil }
func (f *fakeClientStream) CloseSend() error             { return nil }
func (f *fakeClientStream) Context() context.Context {
	if f.ctx != nil {
		return f.ctx
	}
	return context.Background()
}
func (f *fakeClientStream) SendMsg(any) error { return nil }
func (f *fakeClientStream) RecvMsg(any) error { return nil }

type fakeServerStream[T any] struct {
	fakeClientStream
	msgs    []T
	delays  []time.Duration
	idx     int
	recvErr error
}

func (s *fakeServerStream[T]) Recv() (*T, error) {
	if err := s.Context().Err(); err != nil {
		return nil, err
	}
	if s.idx < len(s.delays) {
		time.Sleep(s.delays[s.idx])
	}
	if s.idx >= len(s.msgs) {
		if s.recvErr != nil {
			return nil, s.recvErr
		}
		return nil, io.EOF
	}
	msg := s.msgs[s.idx]
	s.idx++
	return &msg, nil
}

func TestCallServerStream_threeMessages(t *testing.T) {
	t.Parallel()
	const (
		d0 = 8 * time.Millisecond
		d1 = 12 * time.Millisecond
		d2 = 6 * time.Millisecond
	)
	stream := &fakeServerStream[int]{
		msgs:   []int{1, 2, 3},
		delays: []time.Duration{d0, d1, d2},
	}
	got := CallServerStream(context.Background(), func(context.Context) (grpc.ServerStreamingClient[int], error) {
		return stream, nil
	})
	if got.Err != nil {
		t.Fatalf("Err = %v, want nil", got.Err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(got.Messages))
	}
	if len(got.MessageIntervals) != 2 {
		t.Fatalf("len(MessageIntervals) = %d, want 2", len(got.MessageIntervals))
	}
	assertDurationAround(t, "TTFB", got.TTFB, d0, 4*time.Millisecond)
	assertDurationAround(t, "interval[0]", got.MessageIntervals[0], d1, 4*time.Millisecond)
	assertDurationAround(t, "interval[1]", got.MessageIntervals[1], d2, 4*time.Millisecond)
	minTotal := d0 + d1 + d2
	if got.TotalDuration < minTotal {
		t.Fatalf("TotalDuration = %v, want >= %v", got.TotalDuration, minTotal)
	}
}

func TestCallServerStream_zeroMessagesEOF(t *testing.T) {
	t.Parallel()
	stream := &fakeServerStream[int]{}
	got := CallServerStream(context.Background(), func(context.Context) (grpc.ServerStreamingClient[int], error) {
		return stream, nil
	})
	if got.Err != nil {
		t.Fatalf("Err = %v, want nil", got.Err)
	}
	if len(got.Messages) != 0 {
		t.Fatalf("Messages = %v, want empty", got.Messages)
	}
	if got.TTFB != 0 {
		t.Fatalf("TTFB = %v, want 0", got.TTFB)
	}
	if len(got.MessageIntervals) != 0 {
		t.Fatalf("MessageIntervals = %v, want empty", got.MessageIntervals)
	}
}

func TestCallServerStream_recvErrorAfterTwo(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("recv failed")
	stream := &fakeServerStream[int]{
		msgs:    []int{1, 2},
		recvErr: wantErr,
	}
	got := CallServerStream(context.Background(), func(context.Context) (grpc.ServerStreamingClient[int], error) {
		return stream, nil
	})
	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("Err = %v, want %v", got.Err, wantErr)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(got.Messages))
	}
	if len(got.MessageIntervals) != 1 {
		t.Fatalf("len(MessageIntervals) = %d, want 1", len(got.MessageIntervals))
	}
	if got.TotalDuration < 0 {
		t.Fatalf("TotalDuration = %v, want non-negative", got.TotalDuration)
	}
}

func TestCallServerStream_openFnError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("open failed")
	got := CallServerStream(context.Background(), func(context.Context) (grpc.ServerStreamingClient[int], error) {
		return nil, wantErr
	})
	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("Err = %v, want %v", got.Err, wantErr)
	}
	if len(got.Messages) != 0 {
		t.Fatalf("Messages = %v, want empty", got.Messages)
	}
	if got.TTFB != 0 {
		t.Fatalf("TTFB = %v, want 0", got.TTFB)
	}
	if got.TotalDuration < 0 {
		t.Fatalf("TotalDuration = %v, want non-negative", got.TotalDuration)
	}
}

func TestCallServerStream_openFnNilStream(t *testing.T) {
	t.Parallel()
	got := CallServerStream(context.Background(), func(context.Context) (grpc.ServerStreamingClient[int], error) {
		return nil, nil
	})
	if !errors.Is(got.Err, ErrNilStream{}) {
		t.Fatalf("Err = %v, want %v", got.Err, ErrNilStream{})
	}
	if len(got.Messages) != 0 {
		t.Fatalf("Messages = %v, want empty", got.Messages)
	}
	if got.TTFB != 0 {
		t.Fatalf("TTFB = %v, want 0", got.TTFB)
	}
	if got.TotalDuration < 0 {
		t.Fatalf("TotalDuration = %v, want non-negative", got.TotalDuration)
	}
}

func TestCallServerStream_nilMessage(t *testing.T) {
	t.Parallel()
	got := CallServerStream(context.Background(), func(context.Context) (grpc.ServerStreamingClient[int], error) {
		return &nilMessageServerStream{}, nil
	})
	if !errors.Is(got.Err, ErrNilStreamMessage{}) {
		t.Fatalf("Err = %v, want %v", got.Err, ErrNilStreamMessage{})
	}
	if len(got.Messages) != 0 {
		t.Fatalf("Messages = %v, want empty", got.Messages)
	}
}

type nilMessageServerStream struct {
	fakeClientStream
}

func (s *nilMessageServerStream) Recv() (*int, error) {
	return nil, nil
}

func TestCallServerStream_contextCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := CallServerStream(ctx, func(c context.Context) (grpc.ServerStreamingClient[int], error) {
		return &fakeServerStream[int]{
			fakeClientStream: fakeClientStream{ctx: c},
			msgs:             []int{1},
			delays:           []time.Duration{50 * time.Millisecond},
		}, nil
	})
	if got.Err == nil {
		t.Fatal("Err = nil, want context error")
	}
}

func assertDurationAround(t *testing.T, name string, got, want, slack time.Duration) {
	t.Helper()
	if got < want-slack || got > want+slack {
		t.Fatalf("%s = %v, want around %v (+/- %v)", name, got, want, slack)
	}
}

type fakeClientStreamSend[Req, Resp any] struct {
	fakeClientStream
	sendDelay    time.Duration
	closeDelay   time.Duration
	failOnSend   int // 1-based; 0 = never
	msgsSent     int
	closeAndRecv func() (*Resp, error)
}

func (s *fakeClientStreamSend[Req, Resp]) Send(*Req) error {
	s.msgsSent++
	if s.failOnSend > 0 && s.msgsSent == s.failOnSend {
		return errors.New("send failed")
	}
	if s.sendDelay > 0 {
		time.Sleep(s.sendDelay)
	}
	return nil
}

func (s *fakeClientStreamSend[Req, Resp]) CloseAndRecv() (*Resp, error) {
	if s.closeDelay > 0 {
		time.Sleep(s.closeDelay)
	}
	if s.closeAndRecv != nil {
		return s.closeAndRecv()
	}
	var zero Resp
	return &zero, nil
}

func TestCallClientStream_happyPath(t *testing.T) {
	t.Parallel()
	const (
		sendDelay  = 5 * time.Millisecond
		closeDelay = 8 * time.Millisecond
	)
	underlying := &fakeClientStreamSend[int, string]{
		sendDelay:  sendDelay,
		closeDelay: closeDelay,
	}
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return underlying, nil
		},
		func(stream grpc.ClientStreamingClient[int, string]) (string, error) {
			for i := range 3 {
				if err := stream.Send(&i); err != nil {
					return "", err
				}
			}
			resp, err := stream.CloseAndRecv()
			if err != nil {
				return "", err
			}
			return *resp, nil
		},
	)
	if got.Err != nil {
		t.Fatalf("Err = %v, want nil", got.Err)
	}
	if got.MessagesSent != 3 {
		t.Fatalf("MessagesSent = %d, want 3", got.MessagesSent)
	}
	if got.ResponseLatency <= 0 {
		t.Fatalf("ResponseLatency = %v, want positive", got.ResponseLatency)
	}
	if got.SendDuration <= 0 {
		t.Fatalf("SendDuration = %v, want positive", got.SendDuration)
	}
	if got.TotalDuration < got.SendDuration+got.ResponseLatency {
		t.Fatalf("TotalDuration = %v, want >= SendDuration(%v) + ResponseLatency(%v)",
			got.TotalDuration, got.SendDuration, got.ResponseLatency)
	}
	assertDurationAround(t, "ResponseLatency", got.ResponseLatency, closeDelay, 4*time.Millisecond)
}

func TestCallClientStream_sendErrorMidWay(t *testing.T) {
	t.Parallel()
	underlying := &fakeClientStreamSend[int, string]{
		failOnSend: 2,
	}
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return underlying, nil
		},
		func(stream grpc.ClientStreamingClient[int, string]) (string, error) {
			for i := range 3 {
				if err := stream.Send(&i); err != nil {
					return "", err
				}
			}
			resp, err := stream.CloseAndRecv()
			if err != nil {
				return "", err
			}
			return *resp, nil
		},
	)
	if got.Err == nil {
		t.Fatal("Err = nil, want send error")
	}
	if got.MessagesSent != 1 {
		t.Fatalf("MessagesSent = %d, want 1", got.MessagesSent)
	}
	if got.ResponseLatency != 0 {
		t.Fatalf("ResponseLatency = %v, want 0", got.ResponseLatency)
	}
	if got.SendDuration <= 0 {
		t.Fatalf("SendDuration = %v, want positive", got.SendDuration)
	}
}

func TestCallClientStream_firstSendFails(t *testing.T) {
	t.Parallel()
	underlying := &fakeClientStreamSend[int, string]{
		failOnSend: 1,
	}
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return underlying, nil
		},
		func(stream grpc.ClientStreamingClient[int, string]) (string, error) {
			i := 0
			if err := stream.Send(&i); err != nil {
				return "", err
			}
			return "", errors.New("unreachable")
		},
	)
	if got.Err == nil {
		t.Fatal("Err = nil, want send error")
	}
	if got.MessagesSent != 0 {
		t.Fatalf("MessagesSent = %d, want 0", got.MessagesSent)
	}
	if got.ResponseLatency != 0 {
		t.Fatalf("ResponseLatency = %v, want 0", got.ResponseLatency)
	}
	if got.SendDuration <= 0 {
		t.Fatalf("SendDuration = %v, want positive", got.SendDuration)
	}
}

func TestCallClientStream_sendFnEarlyReturn(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("validation failed")
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return &fakeClientStreamSend[int, string]{}, nil
		},
		func(grpc.ClientStreamingClient[int, string]) (string, error) {
			return "", wantErr
		},
	)
	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("Err = %v, want %v", got.Err, wantErr)
	}
	if got.MessagesSent != 0 {
		t.Fatalf("MessagesSent = %d, want 0", got.MessagesSent)
	}
	if got.ResponseLatency != 0 {
		t.Fatalf("ResponseLatency = %v, want 0", got.ResponseLatency)
	}
	if got.SendDuration <= 0 {
		t.Fatalf("SendDuration = %v, want positive", got.SendDuration)
	}
	if got.SendDuration > got.TotalDuration {
		t.Fatalf("SendDuration = %v, want <= TotalDuration %v", got.SendDuration, got.TotalDuration)
	}
}

func TestCallClientStream_zeroSendCloseAndRecv(t *testing.T) {
	t.Parallel()
	const closeDelay = 6 * time.Millisecond
	underlying := &fakeClientStreamSend[int, string]{
		closeDelay: closeDelay,
	}
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return underlying, nil
		},
		func(stream grpc.ClientStreamingClient[int, string]) (string, error) {
			resp, err := stream.CloseAndRecv()
			if err != nil {
				return "", err
			}
			return *resp, nil
		},
	)
	if got.Err != nil {
		t.Fatalf("Err = %v, want nil", got.Err)
	}
	if got.MessagesSent != 0 {
		t.Fatalf("MessagesSent = %d, want 0", got.MessagesSent)
	}
	if got.SendDuration <= 0 {
		t.Fatalf("SendDuration = %v, want positive", got.SendDuration)
	}
	if got.ResponseLatency <= 0 {
		t.Fatalf("ResponseLatency = %v, want positive", got.ResponseLatency)
	}
	if got.TotalDuration < got.SendDuration+got.ResponseLatency {
		t.Fatalf("TotalDuration = %v, want >= SendDuration(%v) + ResponseLatency(%v)",
			got.TotalDuration, got.SendDuration, got.ResponseLatency)
	}
}

func TestCallClientStream_openFnNilStream(t *testing.T) {
	t.Parallel()
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return nil, nil
		},
		func(grpc.ClientStreamingClient[int, string]) (string, error) {
			t.Fatal("sendFn should not run")
			return "", nil
		},
	)
	if !errors.Is(got.Err, ErrNilStream{}) {
		t.Fatalf("Err = %v, want %v", got.Err, ErrNilStream{})
	}
	if got.TotalDuration < 0 {
		t.Fatalf("TotalDuration = %v, want non-negative", got.TotalDuration)
	}
	if got.SendDuration != 0 || got.ResponseLatency != 0 || got.MessagesSent != 0 {
		t.Fatalf("unexpected timings: SendDuration=%v ResponseLatency=%v MessagesSent=%d",
			got.SendDuration, got.ResponseLatency, got.MessagesSent)
	}
}

func TestCallClientStream_openFnError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("open failed")
	got := CallClientStream(context.Background(),
		func(context.Context) (grpc.ClientStreamingClient[int, string], error) {
			return nil, wantErr
		},
		func(grpc.ClientStreamingClient[int, string]) (string, error) {
			t.Fatal("sendFn should not run")
			return "", nil
		},
	)
	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("Err = %v, want %v", got.Err, wantErr)
	}
	if got.TotalDuration < 0 {
		t.Fatalf("TotalDuration = %v, want non-negative", got.TotalDuration)
	}
	if got.SendDuration != 0 || got.ResponseLatency != 0 || got.MessagesSent != 0 {
		t.Fatalf("unexpected timings: SendDuration=%v ResponseLatency=%v MessagesSent=%d",
			got.SendDuration, got.ResponseLatency, got.MessagesSent)
	}
}
