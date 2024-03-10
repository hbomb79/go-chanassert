package chanassert_test

import (
	"testing"
	"time"

	"chanassert"
)

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
