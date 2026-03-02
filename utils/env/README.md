# Env

The env package provides utility functions for working with environment variables in Go.

## Usage

Import the package

```go
import "go.alis.build/utils/env"
```

## Features

### MustGet

`MustGet` returns the value of the required environment variable with the given name. If the variable is not set, it panics with an error message.

```go
	value := env.MustGet("MY_ENV_VAR")
```

### MustExist

`MustExist` checks if multiple environment variables are set. If any are missing, it panics with a list of the missing variables.

```go
	env.MustExist("ALIS_OS_PROJECT", "ALIS_PROJECT_NR", "ALIS_REGION")
```