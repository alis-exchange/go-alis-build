# Long-Running Operations (LROs)

This Go package provides a client for managing long-running operations (LROs) using Google Cloud Spanner and Google Cloud Workflows.

## Background

Long-running operations (LROs) are a pattern in resource-driven design where an operation takes an extended period of
time to complete. This can be due to a number of factors, such as the size of the operation, the amount of data
involved, or the availability of resources.

LROs are often used for tasks such as:

- Copying large amounts of data
- Processing large amounts of data
- Running complex queries
- Executing long-running tasks

LROs can be implemented in a number of ways, but the most common approach is to use a client-server model.
In this model, the client initiates the operation and the server performs the operation in the background.
The client can then poll the server to check the status of the operation until it is complete.

More details on LROs are available at: https://google.aip.dev/151

This package makes managing LROs on your server super simple. 😎

## Requirements
This package is part of the Alis Build Platform and have the following requirements:

- A Google Cloud Spanner database is required and grant permission to the Alis Build Platform service account.
- Enable the `Managed Operations` feature within the Alis Build VS Code extension which will provision the requred Spanner table as well as the underlyging Google Cloud Workflows resource within your deployment.


## Features

 - **Simplified LRO Management**: Create, retrieve, update, and wait for LROs with ease.
 - **Spanner Integration**: Persistently store LRO state in Google Cloud Spanner for reliability.
 - **Resumable LROs**: Leverage Google Cloud Workflows to handle asynchronous waiting and resume operations seamlessly.
 - **Developer-Friendly**: Intuitive API and clear examples for smooth integration into your Go projects.

## Key Concepts:

- **Operation**: Represents an LRO and provides methods for managing its lifecycle.
- **State**: Store and retrieve custom state associated with an LRO, enabling you to resume operations from where they left off.
- **Wait**: Block until an operation is complete, with options for timeouts, polling intervals, and waiting on child operations.
- **Asynchronous Wait**: Delegate long waits to Google Cloud Workflows, freeing up your application resources.

## Getting Started:

1. Installation:

    ```bash
    go get go.alis.build/lro
    ```
    
2.  Initialization:

    ```golang
    import (
        "context"
        "go.alis.build/lro"
        "go.alis.build/sproto" 
    )

    func main() {
        ctx := context.Background()

        // Configure Spanner
        spannerConfig := &lro.SpannerConfig{
            Project:  "your-gcp-project",
            Instance: "your-spanner-instance",
            Database: "your-spanner-database",
        }

        // Create a new LRO client
        client, err := lro.NewClient(ctx, spannerConfig)
        if err != nil {
            // Handle error
        }
        defer client.Close()

        // ... your LRO management logic ...
    }
    ```

2. Example Usage:

    ```golang
    // Create a new LRO
    op, err := lro.NewOperation[any](ctx, client) 
    if err != nil {
        // Handle error
    }

    // ... perform long-running task ...

    // Mark the operation as done
    err = op.Done(&yourResponseProtoMessage)
    if err != nil {
        // Handle error
    }
    ```