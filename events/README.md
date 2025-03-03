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

	// Create a new events Client
	client, err := events.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
		fmt.Println(err)
		return
	}

	// Publish a message as the event.
	var message proto.Message // The message would typically be one of the autogenerated messages, say pb.Portfolio{}
	if err := client.Publish(ctx, message); err != nil {
		// TODO: Handle error.
		fmt.Println(err)
		return
	}

	// Publish a batch of events.
	events := []proto.Message{message, message}
	if err := client.BatchPublish(ctx, events, events.WithJitter(1*time.Second, 5*time.Second)); err != nil {
		// TODO: Handle error.
		fmt.Println(err)
		return
	}
}
```