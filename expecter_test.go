package chanassert_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hbomb79/go-chanassert"
)

type delayMessage[T any] struct {
	delay   time.Duration
	message T
}

type (
	expectedError  int
	expectedErrors []expectedError
)

const (
	rejectedError expectedError = iota
	unsatisfiedError
	terminatedError
)

func (expectedError expectedError) String() string {
	return []string{"rejected error", "unsatisfied error", "terminated error"}[expectedError]
}

func (exp expectedErrors) contains(err expectedError) bool {
	for _, e := range exp {
		if e == err {
			return true
		}
	}

	return false
}

type expecterTest[T any] struct {
	Summary       string
	Messages      []T
	DelayMessages []delayMessage[T]

	// ExpectedErrors contains the error types we expect the expecter to return. These
	// errors can cover all possibilities, such as unexpected messages, timeout cancellation,
	// and unsatisfied layers. An empty slice will enforce that there were no errors returned.
	ExpectedErrors expectedErrors
}

func assertErrorsExpected[T any](t *testing.T, errs chanassert.Errors, expected expectedErrors) {
	if len(errs) == 0 && len(expected) == 0 {
		return
	}

	if len(errs) == 0 && len(expected) > 0 {
		t.Fatalf("no errors returned from expecter, but we expected to see errors: %s", expected)
		return
	}

	// Check that all errors returned are EXPECTED, fail if
	// any errors are not, or if any expected errors were not seen.
	outstanding := make(map[expectedError]struct{})
	for _, e := range expected {
		outstanding[e] = struct{}{}
	}

	for _, err := range errs {
		if expected.contains(rejectedError) {
			rejErr := &chanassert.RejectionError[T]{}
			if errors.As(err, rejErr) {
				delete(outstanding, rejectedError)
				continue
			}
		}

		if expected.contains(unsatisfiedError) {
			unsatisErr := &chanassert.UnsatisfiedError{}
			if errors.As(err, unsatisErr) {
				delete(outstanding, unsatisfiedError)
				continue
			}
		}

		if expected.contains(terminatedError) {
			terminatedErr := &chanassert.TerminatedError{}
			if errors.As(err, terminatedErr) {
				delete(outstanding, terminatedError)
				continue
			}
		}

		t.Errorf("error '%s' returned by expecter, but NOT expected", err)
	}

	if len(outstanding) != 0 {
		t.Errorf("expected errors not satisfied:\n=> Errors returned:\n---\n%s\n=> Expected:\n---\n%s\n=> Did not see:\n---\n%s\n\n", errs, expected, outstanding)
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

			defer func() {
				if !t.Failed() {
					return
				}

				builder := strings.Builder{}
				expecter.FPrintTrace(&builder)
				t.Logf("test failed, expecter trace:\n%s", builder.String())
			}()

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
			assertErrorsExpected[T](t, errs, test.ExpectedErrors)
		})
	}
}

func Test_SingleCombiner(t *testing.T) {
	tests := []expecterTest[string]{
		{
			Summary:        "Expected messages delivered",
			Messages:       []string{"foo", "hello", "world"},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Expected messages delivered, with some unexpected",
			Messages:       []string{"hello", "world", "bar", "foo"},
			ExpectedErrors: []expectedError{rejectedError},
		},
		{
			Summary:        "Insufficient expected messages",
			Messages:       []string{"hello", "world"},
			ExpectedErrors: []expectedError{unsatisfiedError, terminatedError},
		},
		{
			Summary:        "Insufficient expected message, with unexpected messages",
			Messages:       []string{"hello", "bar", "world"},
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
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
			ExpectedErrors: []expectedError{},
		},
		{
			Summary: "Expected messages delivered, but after timeout",
			DelayMessages: []delayMessage[string]{
				{0, "foo"}, {300 * time.Millisecond, "hello"}, {300 * time.Millisecond, "world"},
			},
			ExpectedErrors: []expectedError{unsatisfiedError, rejectedError, terminatedError},
		},
		{
			Summary: "Expected messages delivered, with some unexpected",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"}, {0, "bar"}, {0, "foo"},
			},
			ExpectedErrors: []expectedError{rejectedError},
		},
		{
			Summary: "Insufficient expected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"},
			},
			ExpectedErrors: []expectedError{unsatisfiedError, terminatedError},
		},
		{
			Summary: "Insufficient expected messages, with unexpected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "bar"}, {0, "world"},
			},
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
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
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Expected messages delivered out of order",
			Messages:       []string{"2nd", "foo", "hello", "world"},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Expected messages delivered, with some unexpected",
			Messages:       []string{"hello", "world", "second", "bar", "foo"},
			ExpectedErrors: []expectedError{rejectedError},
		},
		{
			Summary:        "Insufficient expected messages",
			Messages:       []string{"hello", "world"},
			ExpectedErrors: []expectedError{unsatisfiedError, terminatedError},
		},
		{
			Summary:        "Insufficient expected messages for both combiners, with unexpected messages",
			Messages:       []string{"hello", "bar", "world"},
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
		},
		{
			Summary:        "One combiner satisfied",
			Messages:       []string{"world", "second", "hello"},
			ExpectedErrors: []expectedError{terminatedError, unsatisfiedError},
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
			ExpectedErrors: []expectedError{},
		},
		{
			Summary: "Expected messages delivered, but after timeout",
			DelayMessages: []delayMessage[string]{
				{0, "foo"}, {0, "second"}, {300 * time.Millisecond, "hello"}, {300 * time.Millisecond, "world"},
			},
			ExpectedErrors: []expectedError{unsatisfiedError, rejectedError, terminatedError},
		},
		{
			Summary: "Expected messages delivered, with some unexpected",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"}, {0, "bar"}, {0, "foo"}, {0, "2nd"},
			},
			ExpectedErrors: []expectedError{rejectedError},
		},
		{
			Summary: "Insufficient expected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "world"},
			},
			ExpectedErrors: []expectedError{unsatisfiedError, terminatedError},
		},
		{
			Summary: "Insufficient expected messages, with unexpected messages",
			DelayMessages: []delayMessage[string]{
				{0, "hello"}, {0, "bar"}, {0, "world"},
			},
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
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
			ExpectedErrors: []expectedError{rejectedError},
		},
		{
			Summary:        "Expected messages for first combiner only",
			Messages:       []string{"hello", "first", "world"},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Expected messages for second combiner only",
			Messages:       []string{"2nd"},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Expected messages for second combiner only, but matching messages for first",
			Messages:       []string{"hello", "world", "second", "first"},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Not enough messages for first combiner",
			Messages:       []string{"hello", "world"},
			ExpectedErrors: []expectedError{unsatisfiedError, terminatedError},
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
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
		},
		{
			Summary:        "Messages expected in defined order after timeout (B)",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "world"}, {time.Millisecond * 500, "first"}, {0, "2nd"}},
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
		},
		{
			Summary:       "Messages expected in random order",
			DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "2nd"}, {0, "first"}, {0, "world"}},
		},
		{
			Summary:        "Expected messages delivered with unknown messages",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "3rd"}, {0, "world"}, {0, "second"}, {0, "first"}},
			ExpectedErrors: []expectedError{rejectedError},
		},
		{
			Summary:        "Expected messages for first combiner only",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {time.Millisecond * 500, "first"}, {0, "world"}},
			ExpectedErrors: []expectedError{rejectedError, terminatedError, unsatisfiedError},
		},
		{
			Summary:        "Expected messages for second combiner only",
			DelayMessages:  []delayMessage[string]{{0, "2nd"}},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Expected messages for second combiner only, but matching messages for first",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "world"}, {0, "second"}, {0, "first"}},
			ExpectedErrors: []expectedError{},
		},
		{
			Summary:        "Not enough messages for first combiner",
			DelayMessages:  []delayMessage[string]{{0, "hello"}, {0, "world"}},
			ExpectedErrors: []expectedError{unsatisfiedError, terminatedError},
		},
	}

	runExpecterTests(t, makeExpecter, tests)
}

// mockTestingT is a simple helper which allows
// us to enforce that messages contains specified
// substrings were observed as being 'delivered' to the
// mock.
type mockTestingT struct {
	seen []data
}

type data struct {
	message string
	wasErr  bool
}

type dataExpect struct {
	message string
	substr  bool
	err     bool
}

func (mock *mockTestingT) Errorf(format string, args ...any) {
	mock.seen = append(mock.seen, data{message: fmt.Sprintf(format, args...), wasErr: true})
}

func (mock *mockTestingT) Error(args ...any) {
	mock.seen = append(mock.seen, data{message: fmt.Sprintln(args...), wasErr: true})
}

func (mock *mockTestingT) Logf(format string, args ...any) {
	mock.seen = append(mock.seen, data{message: fmt.Sprintf(format, args...), wasErr: false})
}

func (mock *mockTestingT) Log(args ...any) {
	mock.seen = append(mock.seen, data{message: fmt.Sprintln(args...), wasErr: false})
}

func (mock *mockTestingT) assertSeen(t *testing.T, expected []dataExpect) {
	defer func() {
		if t.Failed() {
			t.Logf("TEST FAILED - DEBUG:\nMessages received by mock:\n[%v\n", mock.seen)
		}
	}()

	if len(expected) != len(mock.seen) {
		t.Fatalf("expected to see %d messages on mock, but have seen %d", len(expected), len(mock.seen))
	}

	for idx, exp := range expected {
		actual := mock.seen[idx]

		if actual.wasErr != exp.err {
			t.Errorf("expected 'wasErr' to be %v for message #%d but got '%v'", exp.err, idx, actual.wasErr)
		}

		if !exp.substr && actual.message != exp.message {
			trimmed := strings.TrimSpace(strings.TrimPrefix(actual.message, "expecter error: "))
			if trimmed != exp.message {
				t.Errorf("expected 'message' to be %q for message #%d but got %q", exp.message, idx, actual.message)
			}
		}

		if exp.substr && !strings.Contains(actual.message, exp.message) {
			t.Errorf("expected 'message' to contain %q for message #%d but actual (%q) did not", exp.message, idx, actual.message)
		}
	}
}

func expectUnsatisfied() dataExpect {
	return dataExpect{
		message: "failed to become satisfied: HINT: use .Debug() to enable verbose message tracing",
		err:     true,
	}
}

func expectDebugUnsatisfied() dataExpect {
	return dataExpect{
		message: "failed to become satisfied",
		err:     true,
	}
}

func expectTerminated() dataExpect {
	return dataExpect{
		message: chanassert.TerminatedError{time.Second}.Error(),
		err:     true,
	}
}

func expectLayerUnsatisfied(layerIdx int) dataExpect {
	return dataExpect{
		message: chanassert.UnsatisfiedError{layerIdx}.Error(),
		err:     true,
	}
}

func expectRejection(idx int, msg string, layer int) dataExpect {
	return dataExpect{
		message: fmt.Sprintf("message #%d (%v) was unexpected by layer #%d\nMessage '%+v' - %s", idx, msg, layer, msg, chanassert.Rejected),
		err:     true,
		substr:  true,
	}
}

func expectDebugRejection(idx int, msg string, layer int) dataExpect {
	return dataExpect{
		message: fmt.Sprintf("message #%d (%v) was unexpected by layer #%d", idx, msg, layer),
		err:     true,
		substr:  true,
	}
}

func expectTrace() dataExpect {
	return dataExpect{
		message: "EXPECTER: DEBUG enabled: trace of processed messages follow:",
		substr:  true,
	}
}

func expectErrorHeader(msgCount, layerCount int) dataExpect {
	return dataExpect{
		message: fmt.Sprintf("received messages which could not be matched (%d messages, across %d layers)", msgCount, layerCount),
		substr:  true,
		err:     true,
	}
}

type mockTTest struct {
	Summary       string
	DelayMessages []delayMessage[string]
	ExpectedSeen  []dataExpect
}

func runMockTTests(t *testing.T, makeExpecter func() (chan string, chanassert.Expecter[string]), tests []mockTTest) {
	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			t.Parallel()

			ch, expecter := makeExpecter()
			expecter.Listen()

			for _, m := range test.DelayMessages {
				time.Sleep(m.delay * time.Millisecond)
				ch <- m.message
			}

			mock := &mockTestingT{}
			expecter.AssertSatisfied(mock, time.Second)
			mock.assertSeen(t, test.ExpectedSeen)
		})
	}
}

//nolint:funlen
func Test_AssertSatisfied_DefaultMode(t *testing.T) {
	t.Run("Single layer", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			exp := chanassert.NewChannelExpecter(ch).ExpectTimeout(
				time.Millisecond*500,
				chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"),
				),
			)

			return ch, exp
		}

		tests := []mockTTest{
			{
				Summary:       "No messages delivered",
				DelayMessages: []delayMessage[string]{},
				ExpectedSeen:  []dataExpect{expectTerminated(), expectLayerUnsatisfied(0), expectUnsatisfied()},
			},
			{
				Summary:       "Unknown messages delivered",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectRejection(0, "foo", 0),
					expectRejection(1, "bar", 0),
					expectLayerUnsatisfied(0),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Valid messages delivered",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}},
				ExpectedSeen:  []dataExpect{},
			},
			{
				Summary:       "Unknown messages interlaced with expected",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "hello"}, {0, "bar"}, {0, "world"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectRejection(0, "foo", 0),
					expectRejection(2, "bar", 0),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Only known messages after layer timeout exceeded",
				DelayMessages: []delayMessage[string]{{400, "hello"}, {200, "world"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(1, 1),
					expectTerminated(),
					expectRejection(1, "world", 0),
					expectLayerUnsatisfied(0),
					expectUnsatisfied(),
				},
			},
		}

		runMockTTests(t, makeExpecter, tests)
	})

	t.Run("Multiple layer", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			exp := chanassert.NewChannelExpecter(ch).
				ExpectTimeout(500*time.Millisecond, chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"),
				)).
				ExpectTimeout(500*time.Millisecond, chanassert.OneOf(
					chanassert.MatchEqual("olleh"), chanassert.MatchEqual("dlrow"),
				))

			return ch, exp
		}

		tests := []mockTTest{
			{
				Summary:       "No messages delivered",
				DelayMessages: []delayMessage[string]{},
				ExpectedSeen:  []dataExpect{expectTerminated(), expectLayerUnsatisfied(0), expectUnsatisfied()},
			},
			{
				Summary:       "Unknown messages delivered to layer 0",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectRejection(0, "foo", 0),
					expectRejection(1, "bar", 0),
					expectLayerUnsatisfied(0),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Known messages to layer 0, unknown to layer 1",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectRejection(2, "foo", 1),
					expectRejection(3, "bar", 1),
					expectLayerUnsatisfied(1),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Known messages to layer 0, insufficient to layer 1",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}},
				ExpectedSeen: []dataExpect{
					expectTerminated(),
					expectLayerUnsatisfied(1),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Known messages to layer 0, but after timeout",
				DelayMessages: []delayMessage[string]{{490, "hello"}, {100, "world"}, {0, "dlrow"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectDebugRejection(1, "world", 0),
					expectDebugRejection(2, "dlrow", 0),
					expectLayerUnsatisfied(0),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Known messages to both layers, but layer 1 after timeout",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {550, "dlrow"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(1, 1),
					expectTerminated(),
					expectDebugRejection(2, "dlrow", 1),
					expectLayerUnsatisfied(1),
					expectUnsatisfied(),
				},
			},
			{
				Summary:       "Known messages to both layers",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {0, "dlrow"}},
				ExpectedSeen:  []dataExpect{},
			},
			{
				Summary:       "Unknown messages interlaced with expected",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "hello"}, {0, "bar"}, {0, "world"}, {0, "baz"}, {0, "dlrow"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(3, 2),
					expectRejection(0, "foo", 0),
					expectRejection(2, "bar", 0),
					expectRejection(4, "baz", 1),
					expectUnsatisfied(),
				},
			},
		}

		runMockTTests(t, makeExpecter, tests)
	})
}

//nolint:funlen
func Test_AssertSatisfied_DebugMode(t *testing.T) {
	t.Run("Single layer", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			exp := chanassert.NewChannelExpecter(ch).
				ExpectTimeout(
					time.Millisecond*500,
					chanassert.AllOf(
						chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"),
					),
				).
				Debug()

			return ch, exp
		}

		tests := []mockTTest{
			{
				Summary:       "No messages delivered",
				DelayMessages: []delayMessage[string]{},
				ExpectedSeen:  []dataExpect{expectTerminated(), expectLayerUnsatisfied(0), expectDebugUnsatisfied(), expectTrace()},
			},
			{
				Summary:       "Unknown messages delivered",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectDebugRejection(0, "foo", 0),
					expectDebugRejection(1, "bar", 0),
					expectLayerUnsatisfied(0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Valid messages delivered",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}},
				ExpectedSeen:  []dataExpect{},
			},
			{
				Summary:       "Unknown messages interlaced with expected",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "hello"}, {0, "bar"}, {0, "world"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectDebugRejection(0, "foo", 0),
					expectDebugRejection(2, "bar", 0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Only known messages after layer timeout exceeded",
				DelayMessages: []delayMessage[string]{{400, "hello"}, {200, "world"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(1, 1),
					expectTerminated(),
					expectDebugRejection(1, "world", 0),
					expectLayerUnsatisfied(0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Unknown messages delivered",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectDebugRejection(0, "foo", 0),
					expectDebugRejection(1, "bar", 0),
					expectLayerUnsatisfied(0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Unknown messages interlaced with expected",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "hello"}, {0, "bar"}, {0, "world"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectDebugRejection(0, "foo", 0),
					expectDebugRejection(2, "bar", 0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
		}

		runMockTTests(t, makeExpecter, tests)
	})

	t.Run("Multiple layer", func(t *testing.T) {
		makeExpecter := func() (chan string, chanassert.Expecter[string]) {
			ch := make(chan string, 10)
			exp := chanassert.NewChannelExpecter(ch).
				ExpectTimeout(500*time.Millisecond, chanassert.AllOf(
					chanassert.MatchEqual("hello"), chanassert.MatchEqual("world"),
				)).
				ExpectTimeout(500*time.Millisecond, chanassert.OneOf(
					chanassert.MatchEqual("olleh"), chanassert.MatchEqual("dlrow"),
				)).
				Debug()

			return ch, exp
		}

		tests := []mockTTest{
			{
				Summary:       "No messages delivered",
				DelayMessages: []delayMessage[string]{},
				ExpectedSeen:  []dataExpect{expectTerminated(), expectLayerUnsatisfied(0), expectDebugUnsatisfied(), expectTrace()},
			},
			{
				Summary:       "Unknown messages delivered to layer 0",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectDebugRejection(0, "foo", 0),
					expectDebugRejection(1, "bar", 0),
					expectLayerUnsatisfied(0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Known messages to layer 0, unknown to layer 1",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {0, "foo"}, {0, "bar"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectDebugRejection(2, "foo", 1),
					expectDebugRejection(3, "bar", 1),
					expectLayerUnsatisfied(1),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Known messages to layer 0, insufficient to layer 1",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}},
				ExpectedSeen: []dataExpect{
					expectTerminated(),
					expectLayerUnsatisfied(1),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Known messages to layer 0, but after timeout",
				DelayMessages: []delayMessage[string]{{490, "hello"}, {100, "world"}, {0, "dlrow"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(2, 1),
					expectTerminated(),
					expectDebugRejection(1, "world", 0),
					expectDebugRejection(2, "dlrow", 0),
					expectLayerUnsatisfied(0),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Known messages to both layers, but layer 1 after timeout",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {550, "dlrow"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(1, 1),
					expectTerminated(),
					expectDebugRejection(2, "dlrow", 1),
					expectLayerUnsatisfied(1),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
			{
				Summary:       "Known messages to both layers",
				DelayMessages: []delayMessage[string]{{0, "hello"}, {0, "world"}, {0, "dlrow"}},
				ExpectedSeen:  []dataExpect{},
			},
			{
				Summary:       "Unknown messages interlaced with expected",
				DelayMessages: []delayMessage[string]{{0, "foo"}, {0, "hello"}, {0, "bar"}, {0, "world"}, {0, "baz"}, {0, "dlrow"}},
				ExpectedSeen: []dataExpect{
					expectErrorHeader(3, 2),
					expectDebugRejection(0, "foo", 0),
					expectDebugRejection(2, "bar", 0),
					expectDebugRejection(4, "baz", 1),
					expectDebugUnsatisfied(),
					expectTrace(),
				},
			},
		}

		runMockTTests(t, makeExpecter, tests)
	})
}
