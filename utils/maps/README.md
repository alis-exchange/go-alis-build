# Maps

The maps package provides different utility functions and data structures for working with maps in Go.

## Usage

Import the package

```go
import "go.alis.build/utils/maps"
```

## Features

### OrderedMap

The `OrderedMap` type is a map that maintains the order of keys. It is useful when you need to maintain the order of keys in a map.

Create a new `OrderedMap` instance using `NewOrderedMap`

```go
    orderedMap := maps.NewOrderedMap[string, int]()
	
	orderedMap.Set("a", 1)
	orderedMap.Set("b", 2)
	
	// Get the value for a key
	a,ok := orderedMap.Get("a")
	
	// Get the keys in order
	orderedMap.Range(func(idx int, key string, value int) bool {
        fmt.Println(idx)
        fmt.Println(key)
        fmt.Println(value)
        return true
    })
	
```