# Ordering

The ordering package provides an easy way to convert Common Expression Language (CEL) order by expressions into sproto compatible SortOrder.

The package supports the [AIP-132](https://google.aip.dev/132#ordering) syntax.

## Usage

Create a new Order instance using `NewOrder`

```go
    order, err := ordering.NewOrder("age desc, name asc", ordering.WithDefaultOrder(sproto.SortOrderDesc))
```

Use the `SortOrder` method to convert the CEL order by expression into a sproto compatible SortOrder

```go
    sortOrder := order.SortOrder()
```