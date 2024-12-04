# Set Utils

The set package provides a simple implementation of a set data structure in Go.

## Usage

Import the package

```go
import "go.alis.build/utils/sets"
```

Create a new Set instance using `NewSet`

```go
    set := sets.NewSet(1,2,3) // Type inferred automatically
    set2 := sets.NewSet[string]() // Type specified explicitly
```

Use the `Add` method to add an element to the set

```go
    set.Add(4)
    set2.Add("hello")
```

Use the `Remove` method to remove an element from the set

```go
    set.Remove(4)
```

Use the `Contains` method to check if an element is in the set

```go
    set.Contains(4)
```

Use the `Len` method to get the number of elements in the set

```go
    set.Len()
```