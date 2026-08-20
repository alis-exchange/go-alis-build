package filtering

import (
	"reflect"
)

// convertToConcreteType takes any interface{} and returns it cast to its most specific concrete type.
//
// This function is used to convert parsed parameters from generic []any or map[string]any
// types to their most concrete forms (e.g., []string, []int64) for better Spanner compatibility.
//
// For slices, if all elements are the same type, it converts to a typed slice:
//
//	[]any{"Alice", "Bob"} -> []string{"Alice", "Bob"}
//	[]any{1, 2, 3}        -> []int64{1, 2, 3}
//
// For maps, if all values are the same type, it converts to a typed map:
//
//	map[string]any{"a": 1, "b": 2} -> map[string]int64{"a": 1, "b": 2}
//
// If elements/values have mixed types, the original value is returned unchanged.
func convertToConcreteType(input any) any {
	if input == nil {
		return nil
	}

	value := reflect.ValueOf(input)
	return convertValue(value).Interface()
}

// convertValue recursively converts a reflect.Value to its most concrete type.
//
// It handles:
//   - Slices: Attempts to convert to typed slice via convertSlice
//   - Maps: Attempts to convert to typed map via convertMap
//   - Interfaces: Unwraps to get the underlying concrete value
//   - Other types: Returns unchanged
func convertValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Slice:
		return convertSlice(value)
	case reflect.Map:
		return convertMap(value)
	case reflect.Interface:
		// Unwrap interface{} to get underlying value
		if !value.IsNil() {
			return convertValue(value.Elem())
		}
		return value
	default:
		return value
	}
}

// convertSlice attempts to convert []interface{} to a typed slice if all elements are the same type.
//
// Algorithm:
//  1. Find the concrete type of the first non-nil element
//  2. Verify all subsequent elements have the same type
//  3. If types match, create a new typed slice and copy elements
//  4. If types differ or slice is empty, return original value
//
// Example:
//
//	[]any{"a", "b", "c"} -> []string{"a", "b", "c"}
//	[]any{1, "mixed"}    -> []any{1, "mixed"} (unchanged, mixed types)
func convertSlice(value reflect.Value) reflect.Value {
	if value.Len() == 0 {
		return value
	}

	// Get the concrete type of the first non-nil element
	var commonType reflect.Type
	for i := 0; i < value.Len(); i++ {
		elem := value.Index(i)
		if elem.Kind() == reflect.Interface && !elem.IsNil() {
			elem = elem.Elem()
		}

		if !elem.IsValid() || (elem.Kind() == reflect.Ptr && elem.IsNil()) {
			continue
		}

		elemType := elem.Type()
		if commonType == nil {
			commonType = elemType
		} else if commonType != elemType {
			// Types don't match, return as-is
			return value
		}
	}

	if commonType == nil {
		return value
	}

	// Create new typed slice
	newSlice := reflect.MakeSlice(reflect.SliceOf(commonType), 0, value.Len())

	for i := 0; i < value.Len(); i++ {
		elem := value.Index(i)
		convertedElem := convertValue(elem)

		// Handle nil values
		if !convertedElem.IsValid() || (convertedElem.Kind() == reflect.Ptr && convertedElem.IsNil()) {
			newSlice = reflect.Append(newSlice, reflect.Zero(commonType))
		} else {
			newSlice = reflect.Append(newSlice, convertedElem)
		}
	}

	return newSlice
}

// convertMap attempts to convert map[K]interface{} to map[K]ConcreteType if all values are the same type.
//
// Algorithm:
//  1. Determine the common value type by checking all map values
//  2. If all values have the same type, create a new typed map
//  3. Copy all key-value pairs to the new map
//  4. If types differ or map is empty, return original value
//
// Example:
//
//	map[string]any{"a": 1, "b": 2} -> map[string]int64{"a": 1, "b": 2}
//	map[string]any{"a": 1, "b": "x"} -> map[string]any{...} (unchanged, mixed types)
func convertMap(value reflect.Value) reflect.Value {
	if value.Len() == 0 {
		return value
	}

	keyType := value.Type().Key()
	var commonValueType reflect.Type

	// Determine common value type
	for _, mapKey := range value.MapKeys() {
		mapValue := value.MapIndex(mapKey)
		if mapValue.Kind() == reflect.Interface && !mapValue.IsNil() {
			mapValue = mapValue.Elem()
		}

		if !mapValue.IsValid() || (mapValue.Kind() == reflect.Ptr && mapValue.IsNil()) {
			continue
		}

		valueType := mapValue.Type()
		if commonValueType == nil {
			commonValueType = valueType
		} else if commonValueType != valueType {
			// Types don't match, return as-is
			return value
		}
	}

	if commonValueType == nil {
		return value
	}

	// Create new typed map
	newMapType := reflect.MapOf(keyType, commonValueType)
	newMap := reflect.MakeMap(newMapType)

	for _, mapKey := range value.MapKeys() {
		mapValue := value.MapIndex(mapKey)
		convertedValue := convertValue(mapValue)

		if !convertedValue.IsValid() || (convertedValue.Kind() == reflect.Ptr && convertedValue.IsNil()) {
			newMap.SetMapIndex(mapKey, reflect.Zero(commonValueType))
		} else {
			newMap.SetMapIndex(mapKey, convertedValue)
		}
	}

	return newMap
}
