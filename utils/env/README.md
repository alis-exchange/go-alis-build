# Env

The env package provides utility functions for working with environment variables in Go.

## Usage

Import the package

```go
import "go.alis.build/utils/env"
```

## Features

### GetOrDefault

`GetOrDefault` returns the value of the environment variable with the given name. If it is not set, it returns the provided default value.

```go
	value := env.GetOrDefault("MY_ENV_VAR", "default-value")
```

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

## Best Practices

### Initialization

These `Must` methods should typically be used within `func init()` blocks. This ensures that the application fails fast and panics before runtime if the required environment variables are not correctly configured.

```go
func init() {
    env.MustExist("REQUIRED_VAR_1", "REQUIRED_VAR_2")
}
```
