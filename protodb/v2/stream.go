package protodb

import (
	"io"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamResponse is a generic streaming response used by ResourceTable.Stream.
// Items are added by the table implementation; callers iterate with Next()
// until io.EOF or an error. The producer must call AddItem for each item,
// then Wait() and Close() when done.
type StreamResponse[T interface{}] struct {
	wg  *sync.WaitGroup
	ch  chan T
	err error
}

// NewStreamResponse creates a new StreamResponse for use by ResourceTable.Stream.
func NewStreamResponse[T interface{}]() *StreamResponse[T] {
	return &StreamResponse[T]{
		wg: &sync.WaitGroup{},
		ch: make(chan T),
	}
}

// AddItem sends an item to the stream. The producer must call wg.Add(1) before
// sending; the consumer's Next() calls wg.Done() when it receives the item.
func (r *StreamResponse[T]) AddItem(item T) {
	if r == nil {
		return
	}

	r.wg.Add(1)
	r.ch <- item
}

// SetError records an error and closes the stream. Next() will return this error
// after the channel is drained.
func (r *StreamResponse[T]) SetError(err error) {
	if r == nil {
		return
	}

	r.err = err
	r.Close()
}

// Close closes the stream channel. The producer must call this after sending
// all items (or after SetError); it should be preceded by Wait().
func (r *StreamResponse[T]) Close() {
	if r == nil {
		return
	}

	close(r.ch)
}

// Wait blocks until the consumer has received all items that were added.
// The producer should call Wait() before Close() to avoid closing the channel
// while items are still being sent.
func (r *StreamResponse[T]) Wait() {
	if r == nil {
		return
	}

	r.wg.Wait()
}

// Next gets the next item from the stream.
// It returns io.EOF when the stream is closed.
func (r *StreamResponse[T]) Next() (T, error) {
	if r == nil {
		var zeroValue T
		return zeroValue, status.Error(codes.InvalidArgument, "Stream response is nil")
	}

	// Get the next item from the channel
	item, ok := <-r.ch
	if !ok {
		var zeroValue T
		// Check if there was an error
		if r.err != nil {
			// If there was an error, return it
			return zeroValue, r.err
		}

		// If the channel is closed, return EOF
		return zeroValue, io.EOF
	}

	// Decrement the Wait group
	r.wg.Done()
	// Return the item
	return item, nil
}
