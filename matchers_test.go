package chanassert_test

import (
	"testing"

	"chanassert"
)

type matcherTest[T any] struct {
	summary      string
	value        T
	shouldAccept bool
}

func runMatcherTests[T any](t *testing.T, matcher chanassert.Matcher[T], tests []matcherTest[T]) {
	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()

			res := matcher.DoesMatch(test.value)
			if test.shouldAccept && !res {
				t.Errorf("Matcher REJECTED message '%v', but it was expected to accept", test.value)
			} else if !test.shouldAccept && res {
				t.Errorf("Matcher ACCEPTED message '%v', but it was expected to reject it", test.value)
			}
		})
	}
}

func Test_MatchEqual(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		stringMatcher := chanassert.MatchEqual("hello, world")
		tests := []matcherTest[string]{
			{
				summary:      "Positive match",
				value:        "hello, world",
				shouldAccept: true,
			},
			{
				summary:      "Negative match",
				value:        "hello, world!",
				shouldAccept: false,
			},
			{
				summary:      "Empty message",
				value:        "",
				shouldAccept: false,
			},
		}
		runMatcherTests(t, stringMatcher, tests)
	})

	t.Run("Bool", func(t *testing.T) {
		boolMatcher := chanassert.MatchEqual(true)
		tests := []matcherTest[bool]{
			{
				summary:      "Positive match",
				value:        true,
				shouldAccept: true,
			},
			{
				summary:      "Negative match",
				value:        false,
				shouldAccept: false,
			},
		}
		runMatcherTests(t, boolMatcher, tests)
	})

	t.Run("Int", func(t *testing.T) {
		intMatcher := chanassert.MatchEqual(64)
		tests := []matcherTest[int]{
			{
				summary:      "Positive match",
				value:        64,
				shouldAccept: true,
			},
			{
				summary:      "Negative match",
				value:        6,
				shouldAccept: false,
			},
		}
		runMatcherTests(t, intMatcher, tests)
	})

	t.Run("Simple struct", func(t *testing.T) {
		type s struct {
			name string
			age  int
		}
		intMatcher := chanassert.MatchEqual(s{"harry", 24})
		tests := []matcherTest[s]{
			{
				summary:      "Positive match",
				value:        s{"harry", 24},
				shouldAccept: true,
			},
			{
				summary:      "Negative match",
				value:        s{"bob", 25},
				shouldAccept: false,
			},
		}
		runMatcherTests(t, intMatcher, tests)
	})

	t.Run("Nested struct", func(t *testing.T) {
		type nested struct {
			foo string
		}

		type s struct {
			name   string
			age    int
			nested nested
		}

		nestedStructMatcher := chanassert.MatchEqual(
			s{"harry", 24, nested{"bar"}},
		)
		tests := []matcherTest[s]{
			{
				summary:      "Positive match with nested value populated",
				value:        s{"harry", 24, nested{"bar"}},
				shouldAccept: true,
			},
			{
				summary:      "Correct top-level match with nested value omitted",
				value:        s{"harry", 24, nested{}},
				shouldAccept: false,
			},
			{
				summary:      "Correct top-level match with nested value incorrect",
				value:        s{"harry", 24, nested{"foo"}},
				shouldAccept: false,
			},
			{
				summary:      "Negative match",
				value:        s{"bob", 25, nested{"bar"}},
				shouldAccept: false,
			},
		}
		runMatcherTests(t, nestedStructMatcher, tests)
	})
}

func Test_MatchStringContains(t *testing.T) {
	matcher := chanassert.MatchStringContains("hell")
	tests := []matcherTest[string]{
		{
			summary:      "Exact match",
			value:        "hell",
			shouldAccept: true,
		},
		{
			summary:      "Substring match",
			value:        "xxhellxx",
			shouldAccept: true,
		},
		{
			summary:      "No sub string match",
			value:        "helo",
			shouldAccept: false,
		},
	}
	runMatcherTests(t, matcher, tests)
}

func Test_MatchStruct(t *testing.T) {
	type a struct {
		x string
	}
	type b struct {
		a
		y string
		z []int
	}

	toMatch := b{
		a: a{x: "hello"},
		y: "world",
		z: []int{1, 2, 3},
	}
	matcher := chanassert.MatchStruct(toMatch)
	tests := []matcherTest[b]{
		{
			summary:      "Same struct",
			value:        toMatch,
			shouldAccept: true,
		},
		{
			summary: "Exact match",
			value: b{
				a: a{x: "hello"},
				y: "world",
				z: []int{1, 2, 3},
			},
			shouldAccept: true,
		},
		{
			summary: "Top level mismatch",
			value: b{
				a: a{x: "hello"},
				y: "INVALID",
				z: []int{1, 2, 3},
			},
			shouldAccept: false,
		},
		{
			summary: "Composite struct mismatch",
			value: b{
				a: a{x: "INVALID"},
				y: "world",
				z: []int{1, 2, 3},
			},
			shouldAccept: false,
		},
		{
			summary: "Slice mismatch",
			value: b{
				a: a{x: "hello"},
				y: "world",
				z: []int{3, 2, 1},
			},
			shouldAccept: false,
		},
	}
	runMatcherTests(t, matcher, tests)
}

func Test_MatchStructFields(t *testing.T) {
	t.Run("Testing private fields will always return false", func(t *testing.T) {
		type s struct {
			foo string
		}
		matcher := chanassert.MatchStructFields[s](map[string]any{"foo": "helloworld"})
		res := matcher.DoesMatch(s{foo: "helloworld"})
		if res {
			t.Errorf("expected struct field match to fail due to private fields")
		}
	})

	t.Run("Struct with private fields will not panic unless attempting to test private field", func(t *testing.T) {
		type s struct {
			foo string
			Bar string
		}
		matcher := chanassert.MatchStructFields[s](map[string]any{"Bar": "hi"})
		res := matcher.DoesMatch(s{foo: "helloworld", Bar: "hi"})
		if !res {
			t.Errorf("struct field match failed")
		}
	})

	t.Run("All field values specified", func(t *testing.T) {
		type s struct {
			Name    string
			Age     int
			Numbers []int
		}

		matcher := chanassert.MatchStructFields[any](map[string]any{"Name": "howdy", "Age": 24, "Numbers": []int{1, 2, 3}})
		tests := []matcherTest[any]{
			{
				summary:      "All field values correct",
				value:        s{Name: "howdy", Age: 24, Numbers: []int{1, 2, 3}},
				shouldAccept: true,
			},
			{
				summary:      "Some field values incorrect",
				value:        s{Name: "howdy", Age: 24, Numbers: []int{1, 1, 1}},
				shouldAccept: false,
			},
			{
				summary: "Struct with extra fields",
				value: struct {
					Name    string
					Age     int
					Numbers []int
					Foo     string
				}{"howdy", 24, []int{1, 2, 3}, "bar"},
				shouldAccept: true,
			},
			{
				summary:      "Zero value struct",
				value:        s{},
				shouldAccept: false,
			},
			{
				summary:      "Struct with missing fields",
				value:        struct{ Name string }{"howdy"},
				shouldAccept: false,
			},
		}

		runMatcherTests(t, matcher, tests)
	})

	t.Run("Extra field values specified", func(t *testing.T) {
		type s struct {
			Name string
			Age  int
		}

		matcher := chanassert.MatchStructFields[any](map[string]any{
			"Name": "howdy",
			"Age":  24,
			"This": "doesntexist",
		})
		if res := matcher.DoesMatch(s{Name: "howdy", Age: 24}); res {
			t.Errorf("expected negative match")
		}
	})

	t.Run("Predicate field values", func(t *testing.T) {
		matcher := chanassert.MatchStructFields[any](map[string]any{
			"Name": func(name string) bool { return name == "Harry" },
			"Sex":  "Male",
			"Age":  func(age int) bool { return age > 10 && age < 25 },
		})

		type legal struct {
			Name string
			Sex  string
			Age  int
		}
		type convertibleType struct {
			Name []byte
			Sex  string
			Age  int
		}
		type differingTypes struct {
			Name []byte
			Sex  string
			Age  string
		}
		type missingFields struct {
			Name string
			Sex  string
		}

		tests := []matcherTest[any]{
			{
				summary:      "All field values correct",
				value:        legal{"Harry", "Male", 11},
				shouldAccept: true,
			},
			{
				summary:      "Name field value incorrect",
				value:        legal{"harry", "Male", 11},
				shouldAccept: false,
			},
			{
				summary:      "Age field value incorrect",
				value:        legal{"Harry", "Male", 100},
				shouldAccept: false,
			},
			{
				summary:      "Compatible type difference",
				value:        convertibleType{[]byte{'H', 'a', 'r', 'r', 'y'}, "Male", 24},
				shouldAccept: false,
			},
			{
				summary:      "Incompatible type difference",
				value:        differingTypes{[]byte{'H', 'a', 'r', 'r', 'y'}, "Male", "7"},
				shouldAccept: false,
			},
			{
				summary:      "Missing fields",
				value:        missingFields{"Harry", "Male"},
				shouldAccept: false,
			},
		}

		runMatcherTests(t, matcher, tests)
	})
}

func Test_MatchStructPartial(t *testing.T) {
	type s struct {
		Name    string
		Age     int
		Hobbies []string
	}

	t.Run("All zero value", func(t *testing.T) {
		matcher := chanassert.MatchStructPartial[any](s{})
		tests := []matcherTest[any]{
			{
				summary:      "Message is also zero value",
				value:        s{},
				shouldAccept: true,
			},
			{
				summary:      "Message is populated",
				value:        s{"harry", 11, []string{"sleeping", "eating"}},
				shouldAccept: true,
			},
		}

		runMatcherTests(t, matcher, tests)
	})

	t.Run("All populated", func(t *testing.T) {
		matcher := chanassert.MatchStructPartial[any](s{
			"Harry", 24, []string{"coding", "gaming"},
		})

		tests := []matcherTest[any]{
			{
				summary:      "Message is correctly populated",
				value:        s{"Harry", 24, []string{"coding", "gaming"}},
				shouldAccept: true,
			},
			{
				summary:      "Message is zero value",
				value:        s{},
				shouldAccept: false,
			},
			{
				summary:      "Message is incorrectly populated A",
				value:        s{"Harry", 24, []string{"sleeping", "eating"}},
				shouldAccept: false,
			},
			{
				summary:      "Message is incorrectly populated",
				value:        s{"harry", 11, []string{"coding", "gaming"}},
				shouldAccept: false,
			},
			{
				summary:      "Message contains zero value",
				value:        s{"harry", 11, []string{}},
				shouldAccept: false,
			},
		}

		runMatcherTests(t, matcher, tests)
	})

	t.Run("Some populated", func(t *testing.T) {
		matcher := chanassert.MatchStructPartial[any](s{
			"", 24, []string{"coding", "gaming"},
		})

		tests := []matcherTest[any]{
			{
				summary:      "Message is correctly populated",
				value:        s{"Harry", 24, []string{"coding", "gaming"}},
				shouldAccept: true,
			},
			{
				summary:      "Message is incorrectly populating ignored field",
				value:        s{"HOWDY", 24, []string{"coding", "gaming"}},
				shouldAccept: true,
			},
			{
				summary:      "Message is zero value",
				value:        s{},
				shouldAccept: false,
			},
			{
				summary:      "Message is incorrectly populated",
				value:        s{"", 11, []string{"coding", "gaming"}},
				shouldAccept: false,
			},
			{
				summary:      "Message contains zero value",
				value:        s{"harry", 11, []string{}},
				shouldAccept: false,
			},
		}

		runMatcherTests(t, matcher, tests)
	})
}

func Test_MatchPredicate(t *testing.T) {
	type s struct {
		Name string
		Age  int
	}
	matcher := chanassert.MatchPredicate(func(v s) bool {
		return v.Name == "Harry" && v.Age == 24
	})

	tests := []matcherTest[s]{
		{
			summary:      "Message is correctly populated",
			value:        s{"Harry", 24},
			shouldAccept: true,
		},
		{
			summary:      "Message does not satisfy predicate",
			value:        s{"HOWDY", 28},
			shouldAccept: false,
		},
	}

	runMatcherTests(t, matcher, tests)
}
