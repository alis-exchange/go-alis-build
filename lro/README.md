# Long-Running Operations (LROs)

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