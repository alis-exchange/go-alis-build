# go-alis-build/events

Package events provides a lightweight set of methods which Builders on the Alis Build Platform could use to publish events.

## Installation

```bash
go get go.alis.build/events
```

## Requirements
This package is part of the Alis Build Platform and have the following requirements:

- Enable the `Managed Events` feature within the Alis Build VS Code extension.


## Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"go.alis.build/events"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx := context.Background()

	// Create a new events Client once per process and reuse it — the
	// client caches one pubsub.Publisher per topic, so per-call
	// construct+Stop cycles are avoided.
	client, err := events.NewClient(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer client.Close()

	// Publish a single event. By default Publish blocks on the broker ack.
	var message proto.Message // Typically an auto-generated event message, e.g. pb.EmailCreatedEvent{}.
	if err := client.Publish(ctx, message); err != nil {
		fmt.Println(err)
		return
	}

	// Publish a batch. Note the slice is named `batch` — using `events`
	// here would shadow the imported package and break the WithJitter
	// call below.
	batch := []proto.Message{message, message}
	if err := client.BatchPublish(ctx, batch, events.WithJitter(1*time.Second, 5*time.Second)); err != nil {
		fmt.Println(err)
		return
	}
}
```