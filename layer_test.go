package chanassert_test

import (
	"testing"
	"time"

	"github.com/hbomb79/go-chanassert"
)

func Test_ExpectTimeout_AllOf_MatchStringContains_Overlapping(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Expect(chanassert.AllOf(
				chanassert.MatchStringContains("foo"),
				chanassert.MatchStringContains("foo"),
				chanassert.MatchStringContains("fo"),
				chanassert.MatchStringContains("fooey"),
			))
	}

	tests := []struct {
		summary       string
		messages      []string
		shouldSucceed bool
	}{
		{
			summary:       "Messages delivered in order",
			messages:      []string{"foo", "foo", "fo", "fooey"},
			shouldSucceed: true,
		},
		{
			summary:  "Messages delivered out of order",
			messages: []string{"foo", "fooey", "fo", "foo"},
			// in some sense you'd expect this would pass, but it's not really possible to do a 'most accurate match' using simply string contains.
			// ultimately, the code is doing the right thing by matching 'fooey' to 'foo' (because it does contain the substr), which
			// means when 'foo' comes along, it has no match candidate and fails.
			shouldSucceed: false,
		},
		{
			summary:       "Not enough messages delivered",
			messages:      []string{"foo", "fooey", "fo"},
			shouldSucceed: false,
		},
		{
			summary:       "Duplicate message delivered",
			messages:      []string{"foo", "fooey", "fooey", "fo"},
			shouldSucceed: false,
		},
		{
			summary:       "Invalid message delivered",
			messages:      []string{"foo", "fooey", "fo", "bar"},
			shouldSucceed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()

			ch, exp := makeExpecter()
			exp.Listen()
			for _, msg := range test.messages {
				ch <- msg
			}

			if test.shouldSucceed {
				exp.AssertSatisfied(t, time.Second)
			} else {
				errs := exp.Satisfied(time.Second)
				if len(errs) > 0 {
					t.Logf("satisified returns errors (as expected): %s", errs)
				} else {
					t.Fatalf("no errors returned from test which should have failed")
				}
			}
		})
	}
}

func Test_ExpectTimeout_AllOf_String(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(
				chanassert.MatchEqual("foo"),
				chanassert.MatchEqual("bar"),
			).
			ExpectTimeout(time.Millisecond*500, chanassert.AllOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			)).
			Expect(chanassert.AllOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			))
	}

	tests := []struct {
		summary           string
		messages          []string
		firstMessageDelay time.Duration
		secondBlockDelay  time.Duration
		shouldSucceed     bool
	}{
		{
			summary:           "No delay",
			messages:          []string{"foo", "hello", "world", "world", "hello"},
			firstMessageDelay: 0,
			secondBlockDelay:  0,
			shouldSucceed:     true,
		},
		{
			summary:           "No delay with repeat messages",
			messages:          []string{"foo", "hello", "hello", "world", "hello", "world"},
			firstMessageDelay: 0,
			secondBlockDelay:  0,
			shouldSucceed:     false,
		},
		{
			summary:           "Less than timeout threshold before first message",
			messages:          []string{"hello", "world", "hello", "world"},
			firstMessageDelay: time.Millisecond * 200,
			secondBlockDelay:  0,
			shouldSucceed:     true,
		},
		{
			summary:           "Never delivers first message",
			messages:          []string{},
			firstMessageDelay: 0,
			secondBlockDelay:  0,
			shouldSucceed:     false,
		},
		{
			summary:           "Never delivers second set of messages",
			messages:          []string{"hello", "world", "foo", "hello"},
			firstMessageDelay: 0,
			secondBlockDelay:  0,
			shouldSucceed:     false,
		},
		{
			summary:           "More than timeout delay before first message",
			messages:          []string{"hello", "world", "hello", "world"},
			firstMessageDelay: time.Second,
			secondBlockDelay:  0,
			shouldSucceed:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()
			ch, exp := makeExpecter()
			exp.Listen()
			for i, msg := range test.messages {
				if i == 0 && test.firstMessageDelay != 0 {
					time.Sleep(test.firstMessageDelay)
				}
				if i == 2 && test.secondBlockDelay != 0 {
					time.Sleep(test.secondBlockDelay)
				}
				ch <- msg
			}

			if test.shouldSucceed {
				exp.AssertSatisfied(t, time.Second)
			} else {
				errs := exp.Satisfied(time.Second)
				if len(errs) > 0 {
					t.Logf("satisified returns errors (as expected): %s", errs)
				} else {
					t.Fatalf("no errors returned from test which should have failed")
				}
			}
		})
	}
}

func Test_Expect_AllOf_String(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(
				chanassert.MatchEqual("foo"),
				chanassert.MatchEqual("bar"),
			).
			Expect(chanassert.AllOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			)).
			Expect(chanassert.AllOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			))
	}

	tests := []struct {
		summary       string
		messages      []string
		shouldSucceed bool
	}{
		{
			summary:       "Allowed messages given in valid order A",
			messages:      []string{"hello", "world", "hello", "world"},
			shouldSucceed: true,
		},
		{
			summary:       "Allowed messages given in valid order B",
			messages:      []string{"world", "hello", "hello", "world"},
			shouldSucceed: true,
		},
		{
			summary:       "Allowed messages with ignored messages",
			messages:      []string{"foo", "hello", "bar", "world", "hello", "world"},
			shouldSucceed: true,
		},
		{
			summary:       "Does not provide valid messages before timeout",
			messages:      []string{"hello", "foo", "bar"},
			shouldSucceed: false,
		},
		{
			summary:       "Multiple instances of ignored messages, no valid messages",
			messages:      []string{"foo", "foo", "foo"},
			shouldSucceed: false,
		},
		{
			summary:       "Provides invalid message",
			messages:      []string{"foo", "invalid", "bar"},
			shouldSucceed: false,
		},
		{
			summary:       "Provides duplicate valid message",
			messages:      []string{"foo", "hello", "world", "hello", "hello", "world", "bar"},
			shouldSucceed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()
			ch, exp := makeExpecter()
			exp.Listen()
			for _, msg := range test.messages {
				ch <- msg
			}

			if test.shouldSucceed {
				exp.AssertSatisfied(t, time.Second)
			} else {
				errs := exp.Satisfied(time.Second)
				if len(errs) > 0 {
					t.Logf("satisified returns errors (as expected): %s", errs)
				} else {
					t.Fatalf("no errors returned from test which should have failed")
				}
			}
		})
	}
}

func Test_ExpectTimeout_OneOf_String(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(
				chanassert.MatchEqual("foo"),
				chanassert.MatchEqual("bar"),
			).
			ExpectTimeout(time.Millisecond*500, chanassert.OneOf(
				chanassert.MatchStringContains("hell"),
				chanassert.MatchEqual("world"),
			)).
			Expect(chanassert.OneOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			))
	}

	tests := []struct {
		summary           string
		messages          []string
		firstMessageDelay time.Duration
		secodMessageDelay time.Duration
		shouldSucceed     bool
	}{
		{
			summary:           "No delay",
			messages:          []string{"foo", "xxxhellxx", "world"},
			firstMessageDelay: 0,
			secodMessageDelay: 0,
			shouldSucceed:     true,
		},
		{
			summary:           "Less than timeout threshold before first message",
			messages:          []string{"hello", "world"},
			firstMessageDelay: time.Millisecond * 200,
			secodMessageDelay: 0,
			shouldSucceed:     true,
		},
		{
			summary:           "Never delivers first message",
			messages:          []string{},
			firstMessageDelay: 0,
			secodMessageDelay: 0,
			shouldSucceed:     false,
		},
		{
			summary:           "More than timeout delay before first message",
			messages:          []string{"hello", "world"},
			firstMessageDelay: time.Second,
			secodMessageDelay: 0,
			shouldSucceed:     false,
		},
		{
			summary:           "More than timeout delay after first message",
			messages:          []string{"hello", "world"},
			firstMessageDelay: 0,
			secodMessageDelay: time.Second,
			shouldSucceed:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()
			ch, exp := makeExpecter()
			exp.Listen()
			for i, msg := range test.messages {
				if i == 0 && test.firstMessageDelay != 0 {
					time.Sleep(test.firstMessageDelay)
				}
				if i == 1 && test.secodMessageDelay != 0 {
					time.Sleep(test.secodMessageDelay)
				}
				ch <- msg
			}

			if test.shouldSucceed {
				exp.AssertSatisfied(t, time.Second)
			} else {
				errs := exp.Satisfied(time.Second)
				if len(errs) > 0 {
					t.Logf("satisified returns errors (as expected): %s", errs)
				} else {
					t.Fatalf("no errors returned from test which should have failed")
				}
			}
		})
	}
}

func Test_Expect_OneOf_String(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(
				chanassert.MatchEqual("foo"),
				chanassert.MatchEqual("bar"),
			).
			Expect(chanassert.OneOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			)).
			Expect(chanassert.OneOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			))
	}

	tests := []struct {
		summary       string
		messages      []string
		shouldSucceed bool
	}{
		{
			summary:       "Allowed messages given in valid order A",
			messages:      []string{"hello", "world"},
			shouldSucceed: true,
		},
		{
			summary:       "Allowed messages given in valid order B",
			messages:      []string{"world", "hello"},
			shouldSucceed: true,
		},
		{
			summary:       "Allowed messages with ignored messages",
			messages:      []string{"foo", "hello", "bar", "world"},
			shouldSucceed: true,
		},
		{
			summary:       "Does not provide valid messages before timeout",
			messages:      []string{"hello", "foo", "bar"},
			shouldSucceed: false,
		},
		{
			summary:       "Multiple instances of ignored messages, no valid messages",
			messages:      []string{"foo", "foo", "foo"},
			shouldSucceed: false,
		},
		{
			summary:       "Provides invalid message",
			messages:      []string{"foo", "invalid", "bar"},
			shouldSucceed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()
			ch, exp := makeExpecter()
			exp.Listen()
			for _, msg := range test.messages {
				ch <- msg
			}

			if test.shouldSucceed {
				exp.AssertSatisfied(t, time.Second)
			} else {
				errs := exp.Satisfied(time.Second)
				if len(errs) > 0 {
					t.Logf("satisified returns errors (as expected): %s", errs)
				} else {
					t.Fatalf("no errors returned from test which should have failed")
				}
			}
		})
	}
}

func Test_Expect_OneOf_Struct(t *testing.T) {
	type Message struct {
		Title string
		Body  map[string]int
	}

	ignoreMessage := Message{Title: "ignoreme"}
	validMessageA := Message{Title: "foo", Body: map[string]int{"a": 123}}
	validMessageB := Message{Title: "bar", Body: map[string]int{}}
	validMessageBPartial := Message{Title: "bar", Body: map[string]int{"ignore": 6, "me": 9}}

	makeExpecter := func() (chan Message, chanassert.Expecter[Message]) {
		c := make(chan Message, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(chanassert.MatchStruct(ignoreMessage)).
			Expect(chanassert.OneOf(
				chanassert.MatchStructFields[Message](map[string]any{
					"Title": validMessageA.Title,
					"Body":  validMessageA.Body,
				}),
			)).
			Expect(chanassert.AllOf(
				chanassert.MatchStruct(validMessageA),
				chanassert.MatchStructPartial(Message{Title: validMessageB.Title}), // zero-value for Body
			))
	}

	tests := []struct {
		summary       string
		messages      []Message
		shouldSucceed bool
	}{
		{
			summary:       "Valid messages",
			messages:      []Message{ignoreMessage, validMessageA, validMessageB, validMessageA},
			shouldSucceed: true,
		},
		{
			summary:       "Valid messages for partial matching",
			messages:      []Message{ignoreMessage, validMessageA, validMessageBPartial, validMessageA},
			shouldSucceed: true,
		},
		{
			summary: "Invalid messages",
			messages: []Message{
				{Title: "nonsense"},
				validMessageA,
				ignoreMessage,
				validMessageB,
			},
			shouldSucceed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()
			ch, exp := makeExpecter()
			exp.Listen()
			for _, msg := range test.messages {
				ch <- msg
			}

			if test.shouldSucceed {
				exp.AssertSatisfied(t, time.Second)
			} else {
				errs := exp.Satisfied(time.Second)
				if len(errs) > 0 {
					t.Logf("satisified returns errors (as expected): %s", errs)
				} else {
					t.Fatalf("no errors returned from test which should have failed")
				}
			}
		})
	}
}
