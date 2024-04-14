package chanassert_test

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hbomb79/go-chanassert"
)

const traceDirPath = "testdata/traces/"

//go:embed testdata/traces/**
var expectations embed.FS

func getTestExpectation(t *testing.T) (string, error) {
	name := t.Name()
	contents, err := expectations.ReadFile(traceDirPath + name)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(contents)), nil
}

func assertTraceExpected(t *testing.T, expecter chanassert.Expecter[string]) {
	builder := &strings.Builder{}
	expecter.FPrintTrace(builder)
	actual := builder.String()

	expected, err := getTestExpectation(t)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			name := filepath.Base(t.Name())
			dir := filepath.Dir(t.Name())
			t.Logf("WARN: trace expectation for this test MISSING... Creating (dir=%q name=%q)", dir, name)

			if err := os.MkdirAll(filepath.Join(traceDirPath, dir), os.ModePerm); err != nil {
				t.Fatalf("Failed to automatically create trace expectation testdata dir: %s", err)
			}

			if err := os.WriteFile(filepath.Join(traceDirPath, dir, name), []byte(actual), os.ModePerm); err != nil {
				t.Fatalf("Failed to automatically create trace expectation testdata file: %s", err)
			}

			t.Skipf("SKIPPED: test expectation did not exist, rerun test")
		} else {
			t.Fatalf("failed to open embedded file '%s' for test expectation: %v\nExpecter trace follows:\n%s\n", t.Name(), err, actual)
		}
	}

	actualTrimmed := strings.TrimSpace(actual)
	expectedTrimmed := strings.TrimSpace(expected)
	if expectedTrimmed != actualTrimmed {
		t.Fatalf("Expected message did not match actual:\n-- Expected --\n%s\n\n-- Actual --\n%s\n", expectedTrimmed, actualTrimmed)
	}
}

func runTraceTests(t *testing.T, makeExpecter func() (chan string, chanassert.Expecter[string]), tests []traceTest) {
	for _, data := range tests {
		t.Run(data.Summary, func(t *testing.T) {
			t.Parallel()

			ch, expecter := makeExpecter()
			expecter.Listen()
			for _, m := range data.Messages {
				ch <- m.str
			}

			if data.ShouldBeSatisfied {
				expecter.AssertSatisfied(t, time.Second)
			} else {
				errors := expecter.AwaitSatisfied(time.Second)
				if len(errors) == 0 {
					t.Errorf("expected did NOT fail when expected to")
				}
			}

			results := expecter.ProcessedMessages()
			if len(results) != len(data.Messages) {
				t.Errorf("Number of processed message results (%d) did not match expected (%d)", len(results), len(data.Messages))
			} else {
				for idx, msg := range data.Messages {
					expected := msg.traceStatus
					actual := results[idx].Status

					if expected != actual {
						t.Errorf("Trace status for message #%d (%q) was expected to be %s, but got %s", idx, msg.str, expected, actual)
					}
				}
			}

			assertTraceExpected(t, expecter)
			expecter.PrintTrace()
		})
	}
}

type message struct {
	str         string
	traceStatus chanassert.MessageStatus
}

type traceTest struct {
	Summary           string
	Messages          []message
	ShouldBeSatisfied bool
}

func Test_Trace_MultipleLayer_SingleCombiner(t *testing.T) {
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

	tests := []traceTest{
		{
			Summary: "Expected in order",
			Messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"world", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Expected out of order",
			Messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"world", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Layer 0 unsatisfied",
			Messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"foo", chanassert.Accepted},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Layer 1 unsatisfied",
			Messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"hello", chanassert.Accepted},
			},
			ShouldBeSatisfied: false,
		},
	}

	runTraceTests(t, makeExpecter, tests)
}

func Test_Trace_SingleLayer_MultipleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(chanassert.MatchEqual("ignore")).
			Expect(
				chanassert.AllOf(
					chanassert.MatchEqual("hello"),
					chanassert.MatchEqual("world"),
				),
				chanassert.ExactlyNOfAny(2,
					chanassert.MatchEqual("foo"),
					chanassert.MatchEqual("bar"),
				),
			)
	}

	tests := []traceTest{
		{
			Summary: "Messages delivered in order",
			Messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Messages delivered out of order",
			Messages: []message{
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"world", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Messages delivered with rejections",
			Messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"reject", chanassert.Rejected},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"reject", chanassert.Rejected},
				{"foo", chanassert.Accepted},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Messages for first combiner",
			Messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Too many messages for first combiner",
			Messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"hello", chanassert.Rejected},
				{"world", chanassert.Rejected},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Messages for second combiner",
			Messages: []message{
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"bar", chanassert.Accepted},
			},
			ShouldBeSatisfied: false,
		},
	}

	runTraceTests(t, makeExpecter, tests)
}

//nolint:funlen
func Test_Trace_MultipleLayer_MultipleCombiner(t *testing.T) {
	makeExpecter := func() (chan string, chanassert.Expecter[string]) {
		c := make(chan string, 10)
		return c, chanassert.
			NewChannelExpecter(c).
			Ignore(chanassert.MatchEqual("ignore")).
			Expect(
				chanassert.AllOf(
					chanassert.MatchEqual("hello"),
					chanassert.MatchEqual("world"),
				),
				chanassert.ExactlyNOfAny(2,
					chanassert.MatchEqual("foo"),
					chanassert.MatchEqual("bar"),
				),
			).
			ExpectAny(
				chanassert.OneOf(
					chanassert.MatchEqual("a"),
					chanassert.MatchEqual("b"),
					chanassert.MatchEqual("c"),
				),
				chanassert.OneOf(
					chanassert.MatchEqual("x"),
					chanassert.MatchEqual("y"),
					chanassert.MatchEqual("z"),
				),
			)
	}

	tests := []traceTest{
		{
			Summary: "Messages delivered in order (A)",
			Messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"a", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Messages delivered in order (Z)",
			Messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"z", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Messages delivered out of order",
			Messages: []message{
				// Layer 0
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"world", chanassert.Accepted},
				// Layer 1
				{"b", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Messages delivered with rejections",
			Messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"reject", chanassert.Rejected},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"reject", chanassert.Rejected},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"d", chanassert.Rejected},
				{"y", chanassert.Accepted},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Messages for first combiner of first layer",
			Messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Messages for first combiner of second layer",
			Messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"c", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Messages for second combiner of second layer",
			Messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"x", chanassert.Accepted},
			},
			ShouldBeSatisfied: true,
		},
		{
			Summary: "Too many messages for first combiner",
			Messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"hello", chanassert.Rejected},
				{"world", chanassert.Rejected},
			},
			ShouldBeSatisfied: false,
		},
		{
			Summary: "Messages for first layer, second combiner",
			Messages: []message{
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"bar", chanassert.Accepted},
			},
			ShouldBeSatisfied: false,
		},
	}

	runTraceTests(t, makeExpecter, tests)
}
