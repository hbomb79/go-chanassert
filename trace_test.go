package chanassert_test

import (
	"embed"
	"strings"
	"testing"
	"time"

	"github.com/hbomb79/go-chanassert"
)

//go:embed testdata/traces/**
var expectations embed.FS

func getTestExpectation(t *testing.T) string {
	name := t.Name()
	contents, err := expectations.ReadFile("testdata/traces/" + name)
	if err != nil {
		t.Fatalf("failed to open embedded file '%s' for test expectation: %v", name, err)
	}

	return strings.TrimSpace(string(contents))
}

func assertTraceExpected(t *testing.T, expecter chanassert.Expecter[string], expected string) {
	builder := &strings.Builder{}
	expecter.FPrintTrace(builder)
	actual := builder.String()

	actualTrimmed := strings.TrimSpace(actual)
	expectedTrimmed := strings.TrimSpace(expected)
	if expectedTrimmed != actualTrimmed {
		t.Fatalf("Expected message did not match actual:\n-- Expected --\n%s\n\n-- Actual --\n%s\n", expectedTrimmed, actualTrimmed)
	}
}

func Test_Trace_SingleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(chanassert.MatchEqual("ignore")).
			Expect(chanassert.AllOf(
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
				chanassert.MatchEqual("foo"),
				chanassert.MatchEqual("bar"),
			)).
			Expect(chanassert.ExactlyNOfAny(2,
				chanassert.MatchEqual("hello"),
				chanassert.MatchEqual("world"),
			))
	}

	tests := []struct {
		Summary        string
		Messages       []string
		ShouldComplete bool
	}{
		{
			Summary:        "Expected in order",
			Messages:       []string{"hello", "world", "foo", "bar", "hello", "world", "world"},
			ShouldComplete: true,
		},
		{
			Summary:        "Expected out of order",
			Messages:       []string{"world", "hello", "bar", "foo", "world", "world"},
			ShouldComplete: true,
		},
		{
			Summary:        "Layer 0 unsatisfied",
			Messages:       []string{"world", "hello", "ignore", "foo"},
			ShouldComplete: false,
		},
		{
			Summary:        "Layer 1 unsatisfied",
			Messages:       []string{"world", "hello", "ignore", "foo", "bar", "hello"},
			ShouldComplete: false,
		},
	}

	for _, data := range tests {
		t.Run(data.Summary, func(t *testing.T) {
			t.Parallel()

			ch, expecter := makeExpecter()

			expecter.Listen()
			for _, m := range data.Messages {
				ch <- m
			}

			if data.ShouldComplete {
				expecter.AssertSatisfied(t, time.Second)
			} else {
				errors := expecter.AwaitSatisfied(time.Second)
				if len(errors) == 0 {
					t.Errorf("expected did NOT fail when expected to")
				}
			}

			assertTraceExpected(t, expecter, getTestExpectation(t))
		})
	}
}
