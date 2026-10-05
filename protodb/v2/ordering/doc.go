// Package ordering provides parsing and manipulation of order-by expressions
// for database queries, following the AIP-132 syntax.
//
// Create an Order using NewOrder("field1 desc, field2 asc"), then access
// the parsed columns using Columns() to get an ordered slice, or Invert()
// to flip all sort directions for reverse-order queries.
package ordering
