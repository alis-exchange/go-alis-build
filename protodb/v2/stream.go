package protodb

import (
	"io"
	"sync"
)

// StreamResponse is a response for a stream
// Call Next to get the next item from the stream
type StreamResponse[T interface{}] struct {
	wg  *sync.WaitGroup
	ch  chan T
	err error
}

// NewStreamResponse creates a new StreamResponse
func NewStreamResponse[T interface{}]() *StreamResponse[T] {
	return &StreamResponse[T]{
		wg: &sync.WaitGroup{},
		ch: make(chan T),
	}
}

func (r *StreamResponse[T]) AddItem(item T) {
	// Increment the Wait group
	r.wg.Add(1)
	// Add the item to the channel
	r.ch <- item
}

func (r *StreamResponse[T]) SetError(err error) {
	// Set the error
	r.err = err
	// Close
	r.Close()
}

func (r *StreamResponse[T]) Close() {
	// Close the channel
	close(r.ch)
}

func (r *StreamResponse[T]) Wait() {
	// Wait for the Wait group to be done
	r.wg.Wait()
}

// Next gets the next item from the stream.
// It returns io.EOF when the stream is closed.
func (r *StreamResponse[T]) Next() (T, error) {
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
