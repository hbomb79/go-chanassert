package chanassert

import (
	"fmt"
	"reflect"
	"strings"
)

// Matcher defines a simple value matcher which is used
// to determine if a recevied message matches a particular
// layer.
type Matcher[T any] interface {
	DoesMatch(t T) bool
}

type equalMatcher[T comparable] struct {
	target T
}

func (eqMatch *equalMatcher[T]) DoesMatch(t T) bool {
	return t == eqMatch.target
}

// ComparableEqual returns a matcher which will check if the
// value provided is equal to the messages provided.
func ComparableEqual[T comparable](val T) Matcher[T] {
	return &equalMatcher[T]{target: val}
}

type stringContainsMatcher struct{ target string }

func (contains *stringContainsMatcher) DoesMatch(message string) bool {
	return strings.Contains(message, contains.target)
}

// StringContains returns a matcher which tests if received string messages
// contain the provided substring.
func StringContains(target string) Matcher[string] {
	return &stringContainsMatcher{target: target}
}

type structEqualMatcher[T any] struct{ target T }

func (eqMatch *structEqualMatcher[T]) DoesMatch(t T) bool {
	return reflect.DeepEqual(t, eqMatch.target)
}

// StructMatch returns a matcher which performs a deep-equality
// check (using reflection).
func StructMatch[T any](target T) Matcher[T] {
	return &structEqualMatcher[T]{target: target}
}

// StructPartialMatch returns a matcher which tests that
// all non-zero values inside of the provided struct
// match the same fields inside of the messages received. That is
// to say, a target with a zero-value for a field will NOT check
// if that value is also a zero-value in the messages.
func StructPartialMatch[T any](target T) Matcher[T] {
	fieldValues := make(map[string]any)
	rt := reflect.TypeOf(target)
	rv := reflect.ValueOf(target)

	// Check if t is a struct
	if rt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("StructPartialMatch expects a struct as it's argument, not %T", target))
	}

	// Iterate over struct fields
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Check for non-zero value (excluding interfaces)
		if fieldValue.Kind() != reflect.Interface && !fieldValue.IsZero() {
			fieldValues[field.Name] = fieldValue.Interface()
		}
	}

	return &structFieldMatcher[T]{fieldsAndValues: fieldValues}
}

type structFieldMatcher[T any] struct {
	fieldsAndValues map[string]any
}

func (fieldEqMatch *structFieldMatcher[T]) DoesMatch(t T) bool {
	rt := reflect.TypeOf(t)
	rv := reflect.ValueOf(t)

	// Check if t is a struct
	if rt.Kind() != reflect.Struct {
		return false
	}

	// Iterate over fieldsAndValues and compare with fields in t
	for field, expectedValue := range fieldEqMatch.fieldsAndValues {
		// Get the field value in t
		fieldValue := rv.FieldByName(field)

		// Check if field exists in t
		if !fieldValue.IsValid() {
			return false
		}

		// Check for compatibility of types
		if !fieldValue.Type().AssignableTo(reflect.TypeOf(expectedValue)) {
			return false
		}

		// Check for value equality (using Kind() for more flexibility)
		if fieldValue.Kind() == reflect.Interface && expectedValue == nil { // Handle nil interface expectation
			continue
		} else if !reflect.DeepEqual(fieldValue.Interface(), expectedValue) {
			return false
		}
	}

	// All fields match
	return true
}

// StructFieldMatch returns a matcher which will match messages which
// contain the field values specified. This is achieved via reflection, and
// extra fields in the message is ignored (however a missing field will cause
// a negative match).
func StructFieldMatch[T any](fieldsAndValues map[string]any) Matcher[T] {
	return &structFieldMatcher[T]{fieldsAndValues: fieldsAndValues}
}
