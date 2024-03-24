package chanassert_test

import (
	"testing"

	"github.com/hbomb79/go-chanassert"
)

type combinerTest[T any] struct {
	summary   string
	messages  []T
	expected  []bool
	satisfied []bool
}

func runCombinerTests[T any](t *testing.T, makeCombiner func() chanassert.Combiner[T], tests []combinerTest[T]) {
	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			if len(test.messages) != len(test.expected) {
				t.Fatalf("runCombinerTests len(messages)[%d] != len(expected)[%d]", len(test.messages), len(test.expected))
			}
			if len(test.messages) != len(test.satisfied) {
				t.Fatalf("runCombinerTests len(messages)[%d] != len(satisfied)[%d]", len(test.messages), len(test.satisfied))
			}

			t.Parallel()

			matcher := makeCombiner()
			for i, msg := range test.messages {
				shouldPass := test.expected[i]
				shouldBeSatisfied := test.satisfied[i]
				res, _ := matcher.TryMatch(msg)

				if shouldPass && !res {
					t.Errorf("Combiner REJECTED message '%v' (#%d), but it was expected to accept", msg, i)
				} else if !shouldPass && res {
					t.Errorf("Combiner ACCEPTED message '%v' (#%d), but it was expected to reject it", msg, i)
				}

				isSatisfied := matcher.IsSatisfied()
				if shouldBeSatisfied && !isSatisfied {
					t.Errorf("Combiner NOT SATISFIED (after message '%v' (#%d)), but it was expected to be", msg, i)
				} else if !shouldBeSatisfied && isSatisfied {
					t.Errorf("Combiner SATISFIED (after message '%v' (#%d)), but it was expected to be unsatisfied", msg, i)
				}
			}
		})
	}
}

func Test_OneOf(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.OneOf(
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}

	tests := []combinerTest[string]{
		{
			summary:   "Single message",
			messages:  []string{"hello"},
			expected:  []bool{true},
			satisfied: []bool{true},
		},
		{
			summary:   "Invalid followed by valid",
			messages:  []string{"foo", "bar", "world"},
			expected:  []bool{false, false, true},
			satisfied: []bool{false, false, true},
		},
		{
			summary:   "Multiple messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, false},
			satisfied: []bool{true, true},
		},
	}
	runCombinerTests(t, makeCombiner, tests)
}

func Test_AllOf(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.AllOf(
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}

	tests := []combinerTest[string]{
		{
			summary:   "Provides one of each message",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, true},
		},
		{
			summary:   "Provides valid and invalid messages",
			messages:  []string{"foo", "hello", "bar", "world", "baz"},
			expected:  []bool{false, true, false, true, false},
			satisfied: []bool{false, false, false, true, true},
		},
		{
			summary:   "Provides multiple of same message",
			messages:  []string{"hello", "hello", "world"},
			expected:  []bool{true, false, true},
			satisfied: []bool{false, false, true},
		},
	}
	runCombinerTests(t, makeCombiner, tests)
}

func Test_AtLeastNOfEach(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.AtLeastNOfEach(3,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly N messages",
			messages:  []string{"hello", "world", "hello", "hello", "world", "world"},
			expected:  []bool{true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, false, true},
		},
		{
			summary:   "Greater than N messages",
			messages:  []string{"hello", "hello", "world", "hello", "hello", "world", "world", "world", "hello"},
			expected:  []bool{true, true, true, true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, false, false, true, true, true},
		},
		{
			summary:   "Less than N messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, false},
		},
		{
			summary:   "Only one matcher satisfied",
			messages:  []string{"hello", "hello", "hello", "hello"},
			expected:  []bool{true, true, true, true},
			satisfied: []bool{false, false, false, false},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "FOO", "world", "hello", "BAR", "hello", "world", "world"},
			expected:  []bool{true, false, true, true, false, true, true, true},
			satisfied: []bool{false, false, false, false, false, false, false, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_BetweenNOfEach(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.BetweenNOfEach(3, 5,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly min messages",
			messages:  []string{"hello", "hello", "hello", "world", "world", "world"},
			expected:  []bool{true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, false, true},
		},
		{
			summary:   "Exactly max messages",
			messages:  []string{"hello", "hello", "hello", "hello", "hello", "world", "world", "world", "world", "world"},
			expected:  []bool{true, true, true, true, true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, false, false, false, true, true, true},
		},
		{
			summary:   "Greater than max messages",
			messages:  []string{"hello", "hello", "hello", "hello", "hello", "world", "world", "world", "world", "world", "hello", "world"},
			expected:  []bool{true, true, true, true, true, true, true, true, true, true, false, false},
			satisfied: []bool{false, false, false, false, false, false, false, true, true, true, true, true},
		},
		{
			summary:   "Less than min messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, false},
		},
		{
			summary:   "One matcher within bounds, other too low",
			messages:  []string{"hello", "hello", "hello", "hello", "world"},
			expected:  []bool{true, true, true, true, true},
			satisfied: []bool{false, false, false, false, false},
		},
		{
			summary:   "One matcher within bounds, other too high",
			messages:  []string{"hello", "hello", "hello", "hello", "world", "world", "world", "world", "world", "world"},
			expected:  []bool{true, true, true, true, true, true, true, true, true, false},
			satisfied: []bool{false, false, false, false, false, false, true, true, true, true},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "FOO", "hello", "hello", "hello", "hello", "world", "world", "world", "BAR", "world", "world"},
			expected:  []bool{true, false, true, true, true, true, true, true, true, false, true, true},
			satisfied: []bool{false, false, false, false, false, false, false, false, true, true, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_ExactlyNOfEach(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.ExactlyNOfEach(2,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly N messages",
			messages:  []string{"hello", "world", "hello", "world"},
			expected:  []bool{true, true, true, true},
			satisfied: []bool{false, false, false, true},
		},
		{
			summary:   "Greater than N messages",
			messages:  []string{"hello", "world", "hello", "world", "hello"},
			expected:  []bool{true, true, true, true, false},
			satisfied: []bool{false, false, false, true, true},
		},
		{
			summary:   "Less than N messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, false},
		},
		{
			summary:   "Only one matcher satisfied",
			messages:  []string{"hello", "hello", "hello", "hello"},
			expected:  []bool{true, true, false, false},
			satisfied: []bool{false, false, false, false},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "world", "FOO", "hello", "world", "BAR"},
			expected:  []bool{true, true, false, true, true, false},
			satisfied: []bool{false, false, false, false, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_AtLeastNOf(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.AtLeastNOf(3,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly N messages",
			messages:  []string{"hello", "world", "hello"},
			expected:  []bool{true, true, true},
			satisfied: []bool{false, false, true},
		},
		{
			summary:   "Greater than N messages",
			messages:  []string{"hello", "hello", "world", "hello", "hello", "world", "world", "world", "hello"},
			expected:  []bool{true, true, true, true, true, true, true, true, true},
			satisfied: []bool{false, false, true, true, true, true, true, true, true},
		},
		{
			summary:   "Less than N messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, false},
		},
		{
			summary:   "Only one matcher satisfied",
			messages:  []string{"hello", "hello", "hello", "hello"},
			expected:  []bool{true, true, true, true},
			satisfied: []bool{false, false, true, true},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "FOO", "world", "hello", "BAR", "hello", "world", "world"},
			expected:  []bool{true, false, true, true, false, true, true, true},
			satisfied: []bool{false, false, false, true, true, true, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_BetweenNOf(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.BetweenNOf(3, 5,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly min messages",
			messages:  []string{"hello", "hello", "world"},
			expected:  []bool{true, true, true},
			satisfied: []bool{false, false, true},
		},
		{
			summary:   "Exactly max messages",
			messages:  []string{"hello", "hello", "world", "hello", "hello"},
			expected:  []bool{true, true, true, true, true},
			satisfied: []bool{false, false, true, true, true},
		},
		{
			summary:   "Greater than max messages",
			messages:  []string{"hello", "hello", "hello", "hello", "hello", "world", "world", "world", "world", "world", "hello", "world"},
			expected:  []bool{true, true, true, true, true, false, false, false, false, false, false, false},
			satisfied: []bool{false, false, true, true, true, true, true, true, true, true, true, true},
		},
		{
			summary:   "Less than min messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, false},
		},
		{
			summary:   "One matcher within bounds, other too low",
			messages:  []string{"hello", "hello", "hello", "hello", "world"},
			expected:  []bool{true, true, true, true, true},
			satisfied: []bool{false, false, true, true, true},
		},
		{
			summary:   "One matcher within bounds, other too high",
			messages:  []string{"hello", "hello", "hello", "hello", "world", "world", "world", "world", "world", "world"},
			expected:  []bool{true, true, true, true, true, false, false, false, false, false},
			satisfied: []bool{false, false, true, true, true, true, true, true, true, true},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "FOO", "hello", "hello", "hello", "hello", "world", "world", "world", "BAR", "world", "world"},
			expected:  []bool{true, false, true, true, true, true, false, false, false, false, false, false},
			satisfied: []bool{false, false, false, true, true, true, true, true, true, true, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_ExactlyNOf(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.ExactlyNOf(2,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly N messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, true},
		},
		{
			summary:   "Greater than N messages",
			messages:  []string{"hello", "world", "hello", "world", "hello"},
			expected:  []bool{true, true, false, false, false},
			satisfied: []bool{false, true, true, true, true},
		},
		{
			summary:   "Less than N messages",
			messages:  []string{"hello"},
			expected:  []bool{true},
			satisfied: []bool{false},
		},
		{
			summary:   "Only one matcher satisfied",
			messages:  []string{"hello", "hello", "hello", "hello"},
			expected:  []bool{true, true, false, false},
			satisfied: []bool{false, true, true, true},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "world", "FOO", "hello", "world", "BAR"},
			expected:  []bool{true, true, false, false, false, false},
			satisfied: []bool{false, true, true, true, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_AtLeastNOfAny(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.AtLeastNOfAny(3,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly N messages of first matcher",
			messages:  []string{"hello", "world", "hello", "hello"},
			expected:  []bool{true, true, true, true},
			satisfied: []bool{false, false, false, true},
		},
		{
			summary:   "Exactly N messages of second matcher",
			messages:  []string{"hello", "world", "world", "hello", "world"},
			expected:  []bool{true, true, true, true, true},
			satisfied: []bool{false, false, false, false, true},
		},
		{
			summary:   "Exactly N messages of both",
			messages:  []string{"hello", "hello", "world", "world", "hello", "hello"},
			expected:  []bool{true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, true, true},
		},
		{
			summary:   "Greater than N of first matcher",
			messages:  []string{"hello", "hello", "hello", "hello"},
			expected:  []bool{true, true, true, true},
			satisfied: []bool{false, false, true, true},
		},
		{
			summary:   "Greater than N of second matcher",
			messages:  []string{"world", "world", "world", "world"},
			expected:  []bool{true, true, true, true},
			satisfied: []bool{false, false, true, true},
		},
		{
			summary:   "Greater than N of both matchers",
			messages:  []string{"hello", "world", "hello", "world", "hello", "world", "hello", "world"},
			expected:  []bool{true, true, true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, true, true, true, true},
		},
		{
			summary:   "Less than N messages",
			messages:  []string{"hello", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, false},
		},
		{
			summary:   "Non-matching messages do not affect satisfaction",
			messages:  []string{"hello", "foo", "hello", "world", "world", "hello", "bar", "hello"},
			expected:  []bool{true, false, true, true, true, true, false, true},
			satisfied: []bool{false, false, false, false, false, true, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_BetweenNOfAny(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.BetweenNOfAny(3, 5,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly min messages of first matcher",
			messages:  []string{"hello", "world", "hello", "world", "hello"},
			expected:  []bool{true, true, true, true, true},
			satisfied: []bool{false, false, false, false, true},
		},
		{
			summary:   "Exactly min messages of second matcher",
			messages:  []string{"world", "hello", "world", "hello", "world"},
			expected:  []bool{true, true, true, true, true},
			satisfied: []bool{false, false, false, false, true},
		},
		{
			summary:   "Exactly max messages of first matcher",
			messages:  []string{"hello", "hello", "hello", "world", "world", "world", "hello", "hello"},
			expected:  []bool{true, true, true, true, true, true, true, true},
			satisfied: []bool{false, false, true, true, true, true, true, true},
		},
		{
			summary:   "Exactly max messages of second matcher",
			messages:  []string{"world", "world", "world", "hello", "hello", "hello", "world", "world"},
			expected:  []bool{true, true, true, true, true, true, true, true},
			satisfied: []bool{false, false, true, true, true, true, true, true},
		},
		{
			summary:   "Greater than max messages of first matcher, below min for second matcher",
			messages:  []string{"hello", "hello", "hello", "world", "hello", "hello", "hello"},
			expected:  []bool{true, true, true, true, true, true, false},
			satisfied: []bool{false, false, true, true, true, true, true},
		},
		{
			summary:   "Greater than max messages of second matcher, below min for first matcher",
			messages:  []string{"world", "world", "world", "hello", "hello", "world", "world", "world"},
			expected:  []bool{true, true, true, true, true, true, true, false},
			satisfied: []bool{false, false, true, true, true, true, true, true},
		},
		{
			summary:   "One matcher within bounds, other too low",
			messages:  []string{"hello", "world", "hello", "world", "hello", "hello"},
			expected:  []bool{true, true, true, true, true, true},
			satisfied: []bool{false, false, false, false, true, true},
		},
		{
			summary:   "One matcher within bounds, other too high",
			messages:  []string{"hello", "hello", "hello", "hello", "world", "world", "world", "world", "world", "world"},
			expected:  []bool{true, true, true, true, true, true, true, true, true, false},
			satisfied: []bool{false, false, true, true, true, true, true, true, true, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}

func Test_ExactlyNOfAny(t *testing.T) {
	makeCombiner := func() chanassert.Combiner[string] {
		return chanassert.ExactlyNOfAny(2,
			chanassert.MatchEqual("hello"),
			chanassert.MatchEqual("world"),
		)
	}
	tests := []combinerTest[string]{
		{
			summary:   "Exactly N messages of first matcher",
			messages:  []string{"hello", "hello"},
			expected:  []bool{true, true},
			satisfied: []bool{false, true},
		},
		{
			summary:   "Exactly N messages of second matcher",
			messages:  []string{"world", "world"},
			expected:  []bool{true, true},
			satisfied: []bool{false, true},
		},
		{
			summary:   "Greater than N messages of first matcher, N messages of second",
			messages:  []string{"hello", "world", "hello", "hello", "world"},
			expected:  []bool{true, true, true, false, false},
			satisfied: []bool{false, false, true, true, true},
		},
		{
			summary:   "Less than N messages of first matcher, N messages of second",
			messages:  []string{"hello", "world", "world"},
			expected:  []bool{true, true, true},
			satisfied: []bool{false, false, true},
		},
	}

	runCombinerTests(t, makeCombiner, tests)
}
