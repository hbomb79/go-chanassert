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

// MatchEqual returns a matcher which will check if the
// value provided is equal to the messages provided.
func MatchEqual[T comparable](val T) *equalMatcher[T] {
	return &equalMatcher[T]{target: val}
}

// MatchStringContains returns a matcher which tests if received string messages
// contain the provided substring.
func MatchStringContains(target string) *stringContainsMatcher {
	return &stringContainsMatcher{target: target}
}

// MatchStructFields returns a matcher which will match messages which
// contain the field values specified. This is achieved via reflection, and
// extra fields in the message is ignored (however a missing field will cause
// a negative match).
func MatchStructFields[T any](fieldsAndValues map[string]any) *structFieldMatcher[T] {
	return &structFieldMatcher[T]{fieldsAndValues: fieldsAndValues}
}

// MatchStruct returns a matcher which performs a deep-equality
// check (using reflection).
func MatchStruct[T any](target T) *structEqualMatcher[T] {
	return &structEqualMatcher[T]{target: target}
}

// MatchPredicate returns a matcher which will match messages
// which return true when passed to the predicate provided.
func MatchPredicate[T any](predicate func(T) bool) *predicateMatcher[T] {
	return &predicateMatcher[T]{predicate: predicate}
}

type predicateMatcher[T any] struct{ predicate func(T) bool }

func (predMatcher *predicateMatcher[T]) DoesMatch(message T) bool {
	return predMatcher.predicate(message)
}

// MatchStructPartial returns a matcher which tests that
// all non-zero values inside of the provided struct
// match the same fields inside of the messages received. That is
// to say, a target with a zero-value for a field will NOT check
// if that value is also a zero-value in the messages.
func MatchStructPartial[T any](target T) *structFieldMatcher[T] {
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

type equalMatcher[T comparable] struct{ target T }

func (eqMatch *equalMatcher[T]) DoesMatch(t T) bool {
	return t == eqMatch.target
}

type stringContainsMatcher struct{ target string }

func (contains *stringContainsMatcher) DoesMatch(message string) bool {
	return strings.Contains(message, contains.target)
}

type structEqualMatcher[T any] struct{ target T }

func (eqMatch *structEqualMatcher[T]) DoesMatch(t T) bool {
	return reflect.DeepEqual(t, eqMatch.target)
}

type structFieldMatcher[T any] struct{ fieldsAndValues map[string]any }

func (fieldEqMatch *structFieldMatcher[T]) DoesMatch(t T) bool {
	rt := reflect.TypeOf(t)
	rv := reflect.ValueOf(t)

	if rt.Kind() != reflect.Struct {
		return false
	}

	// Iterate over fieldsAndValues and compare with fields in t
	for field, expectedValue := range fieldEqMatch.fieldsAndValues {
		fieldValue := rv.FieldByName(field)
		if !fieldValue.IsValid() {
			return false
		}

		// If the expectedValue is a function, attempt to call it
		// and return false for this message if the return value is false.
		if reflect.TypeOf(expectedValue).Kind() == reflect.Func {
			funcValue := reflect.ValueOf(expectedValue)
			in := []reflect.Value{fieldValue}
			returnValues := funcValue.Call(in)

			// Expect a single bool return value
			if len(returnValues) != 1 || returnValues[0].Kind() != reflect.Bool {
				panic("Expected matching function to return a single boolean value")
			}

			if !returnValues[0].Bool() {
				return false
			}
			continue
		}

		if !fieldValue.Type().AssignableTo(reflect.TypeOf(expectedValue)) {
			return false
		}

		if fieldValue.Kind() == reflect.Interface && expectedValue == nil {
			continue
		} else if !reflect.DeepEqual(fieldValue.Interface(), expectedValue) {
			return false
		}
	}

	return true
}
