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
		t.Run(data.summary, func(t *testing.T) {
			t.Parallel()

			ch, expecter := makeExpecter()
			expecter.Debug().Listen()
			for _, m := range data.messages {
				ch <- m.str
			}

			if data.shouldBeSatisfied {
				expecter.AssertSatisfied(t, time.Second)
			} else {
				errors := expecter.AwaitSatisfied(time.Second)
				if len(errors) == 0 {
					t.Errorf("expected did NOT fail when expected to")
				}
			}

			results := expecter.ProcessedMessages()
			if len(results) != len(data.messages) {
				t.Errorf("Number of processed message results (%d) did not match expected (%d)", len(results), len(data.messages))
			} else {
				for idx, msg := range data.messages {
					expected := msg.traceStatus
					actual := results[idx].Status

					if expected != actual {
						t.Errorf("Trace status for message #%d (%q) was expected to be %s, but got %s", idx, msg.str, expected, actual)
					}
				}
			}

			assertTraceExpected(t, expecter)
		})
	}
}

type message struct {
	str         string
	traceStatus chanassert.MessageStatus
}

type traceTest struct {
	summary           string
	messages          []message
	shouldBeSatisfied bool
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
			summary: "Expected in order",
			messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"world", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Expected out of order",
			messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"world", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Layer 0 unsatisfied",
			messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"foo", chanassert.Accepted},
			},
			shouldBeSatisfied: false,
		},
		{
			summary: "Layer 1 unsatisfied",
			messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"hello", chanassert.Accepted},
			},
			shouldBeSatisfied: false,
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
			summary: "Messages delivered in order",
			messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Messages delivered out of order",
			messages: []message{
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"world", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Messages delivered with rejections",
			messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"reject", chanassert.Rejected},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"reject", chanassert.Rejected},
				{"foo", chanassert.Accepted},
			},
			shouldBeSatisfied: false,
		},
		{
			summary: "Messages for first combiner",
			messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
			},
			shouldBeSatisfied: false,
		},
		{
			summary: "Too many messages for first combiner",
			messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"hello", chanassert.Rejected},
				{"world", chanassert.Rejected},
			},
			shouldBeSatisfied: false,
		},
		{
			summary: "Messages for second combiner",
			messages: []message{
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"bar", chanassert.Accepted},
			},
			shouldBeSatisfied: false,
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
			summary: "Messages delivered in order (A)",
			messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"a", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Messages delivered in order (Z)",
			messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"z", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Messages delivered out of order",
			messages: []message{
				// Layer 0
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"world", chanassert.Accepted},
				// Layer 1
				{"b", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Messages delivered with rejections",
			messages: []message{
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
			shouldBeSatisfied: false,
		},
		{
			summary: "Messages for first combiner of first layer",
			messages: []message{
				{"world", chanassert.Accepted},
				{"hello", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
			},
			shouldBeSatisfied: false,
		},
		{
			summary: "Messages for first combiner of second layer",
			messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"c", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Messages for second combiner of second layer",
			messages: []message{
				// Layer 0
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"foo", chanassert.Accepted},
				// Layer 1
				{"x", chanassert.Accepted},
			},
			shouldBeSatisfied: true,
		},
		{
			summary: "Too many messages for first combiner",
			messages: []message{
				{"hello", chanassert.Accepted},
				{"world", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"hello", chanassert.Rejected},
				{"world", chanassert.Rejected},
			},
			shouldBeSatisfied: false,
		},
		{
			summary: "Messages for first layer, second combiner",
			messages: []message{
				{"foo", chanassert.Accepted},
				{"bar", chanassert.Accepted},
				{"ignore", chanassert.Ignored},
				{"bar", chanassert.Accepted},
			},
			shouldBeSatisfied: false,
		},
	}

	runTraceTests(t, makeExpecter, tests)
}
