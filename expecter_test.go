package chanassert_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hbomb79/go-chanassert"
)

type delayMessage[T any] struct {
	delay   time.Duration
	message T
}

type expecterTest[T any] struct {
	Summary       string
	Messages      []T
	DelayMessages []delayMessage[T]

	// ExpectedErrors contains the wrapped errors we expect the expecter to return. These
	// errors can cover all possibilities, such as unexpected messages, timeout cancellation,
	// and unsatisfied layers. An empty slice will enforce that there were no errors returned.
	ExpectedErrors []error
}

func assertErrorsExpected(t *testing.T, errs chanassert.Errors, expected []error) {
	if len(errs) == 0 && len(expected) == 0 {
		return
	}

	if len(errs) == 0 && len(expected) > 0 {
		t.Fatalf("no errors returned from expecter, but we expected to see errors: %s", expected)
		return
	}

	for _, err := range errs {
		found := false
		for _, expErr := range expected {
			if errors.Is(err, expErr) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("error '%s' returned by expecter, but was NOT expected", err)
		}
	}
}

func runExpecterTests[T any](t *testing.T, makeExpecter func() (chan T, chanassert.Expecter[T]), tests []expecterTest[T]) {
	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			t.Parallel()

			if test.Messages != nil && test.DelayMessages != nil {
				t.Fatalf("test has defined both Messages and DelayMessages, however only one can be defined")
			} else if test.Messages == nil && test.DelayMessages == nil {
				t.Fatalf("test has defined neither Messages nor DelayMessages")
			}

			ch, expecter := makeExpecter()
			expecter.Listen()

			if test.Messages != nil {
				for _, m := range test.Messages {
					ch <- m
				}
			} else {
				for _, m := range test.DelayMessages {
					time.Sleep(m.delay)
					ch <- m.message
				}
			}

			errs := expecter.AwaitSatisfied(time.Second)
			assertErrorsExpected(t, errs, test.ExpectedErrors)
		})
	}
}

func Test_SingleCombiner(t *testing.T) {
	tests := []expecterTest[string]{
		{
			Summary:        "Expected messages delivered",
			Messages:       []string{"foo", "hello", "world"},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Expected messages delivered, with some unexpected",
			Messages:       []string{"hello", "world", "bar", "foo"},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage},
		},
		{
			Summary:        "Insufficient expected messages",
			Messages:       []string{"hello", "world"},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrUnsatisfiedExpecter},
		},
		{
			Summary:        "Insufficient expected message, with unexpected messages",
			Messages:       []string{"hello", "bar", "world"},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
	}

	t.Run("Expect", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			expecter := chanassert.NewChannelExpecter(ch).Expect(
				chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("foo"),
				),
			)

			return ch, expecter
		}

		runExpecterTests(t, makeExpecter, tests)
	})

	t.Run("ExpectAny", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			expecter := chanassert.NewChannelExpecter(ch).ExpectAny(
				chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("foo"),
				),
			)

			return ch, expecter
		}
		runExpecterTests(t, makeExpecter, tests)
	})
}

func Test_SingleCombiner_Timeout(t *testing.T) {
	tests := []expecterTest[string]{
		{
			Summary: "Expected messages delivered",
			DelayMessages: []delayMessage[string]{
				{0, "foo"}, {0, "hello"}, {0, "world"},
			},
			ExpectedErrors: []error{},
		},
		{
			Summary: "Expected messages delivered, but after timeout",
			DelayMessages: []delayMessage[string]{
				{0, "foo"}, {300 * time.Millisecond, "hello"}, {300 * time.Millisecond, "world"},
			},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter},
		},
		{
			Summary: "Expected messages delivered, with some unexpected",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"}, {0, "bar"}, {0, "foo"},
			},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage},
		},
		{
			Summary: "Insufficient expected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"},
			},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrUnsatisfiedExpecter},
		},
		{
			Summary: "Insufficient expected messages, with unexpected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "bar"}, {0, "world"},
			},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
	}

	t.Run("Expect", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			expecter := chanassert.NewChannelExpecter(ch).ExpectTimeout(
				time.Millisecond*500,
				chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("foo"),
				),
			)

			return ch, expecter
		}

		runExpecterTests(t, makeExpecter, tests)
	})

	t.Run("ExpectAny", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			expecter := chanassert.NewChannelExpecter(ch).ExpectAnyTimeout(
				time.Millisecond*500,
				chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("foo"),
				),
			)

			return ch, expecter
		}

		runExpecterTests(t, makeExpecter, tests)
	})
}

func Test_Expect_MultipleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		ch := make(chan string, 10)
		expecter := chanassert.NewChannelExpecter(ch).Expect(
			chanassert.AllOf(
				chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("foo"),
			),
			chanassert.OneOf(
				chanassert.MatchEqual("second"), chanassert.MatchEqual("2nd"),
			),
		)

		return ch, expecter
	}

	tests := []expecterTest[string]{
		{
			Summary:        "Expected messages delivered",
			Messages:       []string{"foo", "hello", "world", "second"},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Expected messages delivered out of order",
			Messages:       []string{"2nd", "foo", "hello", "world"},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Expected messages delivered, with some unexpected",
			Messages:       []string{"hello", "world", "second", "bar", "foo"},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage},
		},
		{
			Summary:        "Insufficient expected messages",
			Messages:       []string{"hello", "world"},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrUnsatisfiedExpecter},
		},
		{
			Summary:        "Insufficient expected messages for both combiners, with unexpected messages",
			Messages:       []string{"hello", "bar", "world"},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
		{
			Summary:        "One combiner satisfied",
			Messages:       []string{"second"},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
	}

	runExpecterTests(t, makeExpecter, tests)
}

func Test_ExpectTimeout_MultipleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		ch := make(chan string, 10)
		expecter := chanassert.NewChannelExpecter(ch).ExpectTimeout(
			time.Millisecond*500,
			chanassert.AllOf(
				chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("foo"),
			),
			chanassert.OneOf(
				chanassert.MatchEqual("second"), chanassert.MatchEqual("2nd"),
			),
		)

		return ch, expecter
	}

	tests := []expecterTest[string]{
		{
			Summary: "Expected messages delivered",
			DelayMessages: []delayMessage[string]{
				{0, "foo"}, {0, "hello"}, {0, "world"}, {0, "second"},
			},
			ExpectedErrors: []error{},
		},
		{
			Summary: "Expected messages delivered, but after timeout",
			DelayMessages: []delayMessage[string]{
				{0, "foo"}, {0, "second"}, {300 * time.Millisecond, "hello"}, {300 * time.Millisecond, "world"},
			},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter},
		},
		{
			Summary: "Expected messages delivered, with some unexpected",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"}, {0, "bar"}, {0, "foo"}, {0, "2nd"},
			},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage},
		},
		{
			Summary: "Insufficient expected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"},
			},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrUnsatisfiedExpecter},
		},
		{
			Summary: "Insufficient expected messages, with unexpected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "bar"}, {0, "world"},
			},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
	}

	runExpecterTests(t, makeExpecter, tests)
}

func Test_ExpectAny_MultipleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		ch := make(chan string, 10)
		exp := chanassert.NewChannelExpecter(ch).ExpectAny(
			chanassert.AllOf(
				chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("first"),
			),
			chanassert.OneOf(
				chanassert.MatchEqual("second"), chanassert.MatchEqual("2nd"),
			),
		)

		return ch, exp
	}

	tests := []expecterTest[string]{
		{
			Summary:  "Messages expected in defined order (A)",
			Messages: []string{"hello", "world", "first", "second"},
		},
		{
			Summary:  "Messages expected in defined order (B)",
			Messages: []string{"hello", "world", "first", "2nd"},
		},
		{
			Summary:  "Messages expected in random order",
			Messages: []string{"hello", "2nd", "first", "world"},
		},
		{
			Summary:        "Expected messages delivered with unknown messages",
			Messages:       []string{"hello", "3rd", "world", "second", "first"},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage},
		},
		{
			Summary:        "Expected messages for first combiner only",
			Messages:       []string{"hello", "first", "world"},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Expected messages for second combiner only",
			Messages:       []string{"2nd"},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Expected messages for second combiner only, but matching messages for first",
			Messages:       []string{"hello", "world", "second", "first"},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Not enough messages for first combiner",
			Messages:       []string{"hello", "world"},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrUnsatisfiedExpecter},
		},
	}

	runExpecterTests(t, makeExpecter, tests)
}

func Test_ExpectAnyTimeout_MultipleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		ch := make(chan string, 10)
		exp := chanassert.NewChannelExpecter(ch).ExpectAnyTimeout(
			time.Millisecond*500,
			chanassert.AllOf(
				chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"), chanassert.MatchEqual("first"),
			),
			chanassert.OneOf(
				chanassert.MatchEqual("second"), chanassert.MatchEqual("2nd"),
			),
		)

		return ch, exp
	}

	tests := []expecterTest[string]{
		{
			Summary:       "Messages expected in defined order (A)",
			DelayMessages: []delayMessage[string]{{0, "hello"}, {time.Millisecond * 400, "world"}, {0, "first"}, {0, "second"}},
		},
		{
			Summary:       "Messages expected in defined order (B)",
			DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {time.Millisecond * 400, "first"}, {0, "2nd"}},
		},
		{
			Summary:        "Messages expected in defined order after timeout (A)",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {time.Millisecond * 500, "world"}, {0, "first"}, {0, "second"}},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
		{
			Summary:        "Messages expected in defined order after timeout (B)",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "world"}, {time.Millisecond * 500, "first"}, {0, "2nd"}},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
		{
			Summary:       "Messages expected in random order",
			DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "2nd"}, {0, "first"}, {0, "world"}},
		},
		{
			Summary:        "Expected messages delivered with unknown messages",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "3rd"}, {0, "world"}, {0, "second"}, {0, "first"}},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage},
		},
		{
			Summary:        "Expected messages for first combiner only",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {time.Millisecond * 500, "first"}, {0, "world"}},
			ExpectedErrors: []error{chanassert.ErrRejectedMessage, chanassert.ErrUnsatisfiedExpecter, chanassert.ErrActiveLayerUnsatisfied},
		},
		{
			Summary:        "Expected messages for second combiner only",
			DelayMessages:  []delayMessage[string]{{0, "2nd"}},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Expected messages for second combiner only, but matching messages for first",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "world"}, {0, "second"}, {0, "first"}},
			ExpectedErrors: []error{},
		},
		{
			Summary:        "Not enough messages for first combiner",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "world"}},
			ExpectedErrors: []error{chanassert.ErrActiveLayerUnsatisfied, chanassert.ErrUnsatisfiedExpecter},
		},
	}

	runExpecterTests(t, makeExpecter, tests)
}
