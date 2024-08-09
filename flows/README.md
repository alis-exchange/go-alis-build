# Flows

This package makes handling flows super simple. 😎

**Flow**: The "Flow" represents the entire journey of an API request, from its initial receipt to the final response generation. It encapsulates the sequence of operations and transformations that occur during the processing of the request.

**Step**: A "Step" is a single, distinct action or operation that contributes to the overall Flow. It's a building block of the larger process, representing a specific stage or task within the API's business logic.

## Installation

Get the package

```bash
go get go.alis.build/flows
```

Import the package

```go
import "go.alis.build/flows"
```

Create a new Sproto instance using `NewClient`

```go
// Create new client
client, err := NewClient(project, WithTopic("flows"))
if err != nil {}
```

## Usage

Create a new flow.

```go
flow, err := client.NewFlow(ctx)
if err != nil {}
```

Add a step to the flow.

```go
step := flow.NewStep("1.0", "Step 1")
```

Set step state

```go
step = step.Queued()
```

Publish the flow

```go
err := flow.Publish()
```