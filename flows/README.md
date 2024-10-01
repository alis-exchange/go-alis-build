# Flows

This package makes handling flows super simple. 😎

**Flow**: The "Flow" represents the entire journey of an API request, from its initial receipt to the final response generation. It encapsulates the sequence of operations and transformations that occur during the processing of the request.

**Step**: A "Step" is a single, distinct action or operation that contributes to the overall Flow. It's a building block of the larger process, representing a specific stage or task within the API's business logic.

When `Publish` is called on a flow/step, the Flow is published to a default "flows" pubsub topic. Users can subscribe to this topic to receive updates on the flow/step.
The published message will include a `type` attribute of value "alis.open.flows.v1.Flow" that can be used as a filter on the subscription.
The default topic can be overridden using the `WithTopic` option.

## Installation

Get the package

```bash
go get go.alis.build/flows
```

Import the package

```go
import "go.alis.build/flows"
```

Create a new Client instance using `NewClient`

```go
// Create new client
client, err := flows.NewClient(gcpProject, flows.WithTopic("flows"), flows.WithAwaitPublish())
if err != nil {}
```

`WithTopic` and `WithAwaitPublish` are optional configurations.

## Usage

Create a new flow.

```go
flow, err := client.NewFlow(ctx)
if err != nil {}
```

Add a step to the flow.

```go
step, ctx, err := flow.NewStep("1.0", flows.WithTitle("Step 1"), flows.WithExistingId())
if err != nil {}
```

`WithExistingId` is an optional configuration.

Set step state

```go
step = step.Queued()
```

Publish the flow

```go
err := flow.Publish()
```