// Package filtering parses AIP-160 filter expressions. Parser.Parse converts
// a filter into a Spanner SQL statement; Parser.Compile evaluates the same
// filter in memory, for adapters that cannot run SQL.
package filtering
