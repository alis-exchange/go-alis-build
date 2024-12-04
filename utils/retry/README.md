# Retry

The retry package provides a simple way to retry a function until it succeeds or the maximum number of attempts is reached.

## Usage

Import the package

```go
import "go.alis.build/utils/retry"
```

Use the `Retry` function to retry a function until it succeeds or the maximum number of attempts is reached.

```go
    result, err := Retry(3, 5*time.Second, func() (interface{}, error) {
        res, err := someFunction()
        if err != nil {
            if err == someError {
                return nil, NewNonRetryableError(err)
            }
            
            return nil, err
        }
        
        return res, nil
    })
```