// chanassert offers an expressive and dynamic
// way to assert that messages over a channel arrive
// as expected.
package chanassert

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	// ErrUnsatisfiedExpecter indicates that the expecter did not 'finish' on
	// it's own before the timeout provided to this function.
	ErrUnsatisfiedExpecter = errors.New("expecter did not become satisfied within timeout")

	// ErrRejectedMessage indicates a message was received by the expecter
	// but could not be matched against the active layer.
	ErrRejectedMessage = errors.New("expecter received message which was rejected")

	// ErrActiveLayerUnsatisfied indicates that the active layer at the time
	// of the expecter finishing was not satisfied. In the absence of an
	// UnsatisfiedExpecterErr, this indicates the message channel closed before the expecter
	// witnessed enough messages to satisfy it's expectations.
	ErrActiveLayerUnsatisfied = errors.New("active layer of expecter did not become satisfied")
)

type Combiner[T any] interface {
	TryMatch(t T) (bool, TraceMessage)
	IsSatisfied() bool
}

// Layer is the highest-level of matcher abstraction, and it defines
// a specific matcher which can be selected by the expecter at any one time. Each
// time a new ExpectMatcher is provided to the Expecter, it is placed in it's own
// layer. This is what gives the expecter the "expect this THEN this THEN this" pattern.
type Layer[T any] interface {
	// TryMatch is called by the Expecter on a layer when a message is received. This method
	// must return true if the message is valid for the layer, otherwise false. A 'Valid' message is
	// determined by the specific layer implementation (e.g. timeoutLayer)
	TryMatch(message T) (bool, TraceMessage)

	// IsSatisfied must return true if the layer does not intend to accept any more messages. Typically
	// a layer will NOT return true if it's in an error condition, however this is implementation specific.
	// If a layer advertsises itself as 'Satisfied' following a successful match via 'DoesMatch', then
	// the next layer in the expecter will be selected.
	IsSatisfied() bool

	// Begin indicates to a layer that it has been selected. This method will be
	// called repeatedly, so a layer must only react to it the first time.
	Begin()
}

type RejectionError[T any] struct {
	MessageNum    int
	MessageResult MessageResult[T]
}

func (e RejectionError[T]) Error() string {
	return fmt.Sprintf(
		"message #%d (%v) was unexpected by layer #%d",
		e.MessageNum,
		e.MessageResult.Message,
		e.MessageResult.LayerIdx,
	)
}

type TerminatedError struct {
	Timeout time.Duration
}

func (e TerminatedError) Error() string {
	return fmt.Sprintf("expecter did not finish within the %s timeout specified", e.Timeout.String())
}

type UnsatisfiedError struct {
	ActiveLayerIdx int
}

func (e UnsatisfiedError) Error() string {
	return fmt.Sprintf("active layer (layer #%d) never became satisfied", e.ActiveLayerIdx)
}

type Errors []error

func (errs Errors) String() string {
	str := strings.Builder{}
	str.WriteString("ExpecterErrors {\n")
	for _, e := range errs {
		str.WriteString("  - ")
		str.WriteString(e.Error())
		str.WriteString("\n")
	}
	str.WriteString("}\n")

	return str.String()
}

type Expecter[T any] interface {
	ExpectAny(combiners ...Combiner[T]) Expecter[T]
	Expect(combiners ...Combiner[T]) Expecter[T]
	ExpectAnyTimeout(timeout time.Duration, combiners ...Combiner[T]) Expecter[T]
	ExpectTimeout(timeout time.Duration, combiners ...Combiner[T]) Expecter[T]

	Ignore(matchers ...Matcher[T]) Expecter[T]

	AssertSatisfied(t TestingT, timeout time.Duration)
	AwaitSatisfied(timeout time.Duration) Errors

	PrintTrace()
	FPrintTrace(w io.Writer)
	ProcessedMessages() []MessageResult[T]

	Debug() Expecter[T]

	Listen()
}

type expecter[T any] struct {
	channel           chan T
	ignoreMatchers    []Matcher[T]
	currentLayerIndex int
	expectLayers      []Layer[T]
	wg                *sync.WaitGroup
	closeChan         chan struct{}
	results           []MessageResult[T]
	debug             bool
}

func NewChannelExpecter[T any](channel chan T) *expecter[T] {
	return &expecter[T]{
		channel:           channel,
		currentLayerIndex: 0,
		ignoreMatchers:    make([]Matcher[T], 0),
		expectLayers:      make([]Layer[T], 0),
		closeChan:         make(chan struct{}, 1),
		wg:                &sync.WaitGroup{},
		results:           make([]MessageResult[T], 0),
	}
}

// Ignore adds a matcher to this expecter which will be
// checked for all incoming messages. Any message which
// is accepted by the ignore matcher(s) will be
// discarded/ignored (NOT rejected).
func (exp *expecter[T]) Ignore(matchers ...Matcher[T]) Expecter[T] {
	exp.ignoreMatchers = append(exp.ignoreMatchers, matchers...)
	return exp
}

func (exp *expecter[T]) addLayer(mode LayerMode, timeout *time.Duration, combiners []Combiner[T]) Expecter[T] {
	layer := &layer[T]{
		mode:      mode,
		layerIdx:  len(exp.expectLayers),
		combiners: combiners,
		timeout:   timeout,
	}

	exp.expectLayers = append(exp.expectLayers, layer)
	return exp
}

// ExpectAny adds a layer to this expecter with some number of combiners. The layer
// will be in 'OR' mode, which means it will become satisfied once ANY of the combiners
// becomes satisfied (versus with [Expect] or [ExpectTimeout], where ALL combiners must
// become satisfied).
func (exp *expecter[T]) ExpectAny(combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(or, nil, combiners)
}

// ExpectAnyTimeout adds a layer to this expecter with some number of combiners. The layer
// will be in 'OR' mode, which means it will become satisfied once ANY of the combiners
// becomes satisfied (versus with [Expect] or [ExpectTimeout], where ALL combiners must
// become satisfied).
//
// This layer will be created with a timeout. Once the layer is selected, the timeout will
// begin and messages delivered to the layer will be rejected once the timeout has elapsed.
func (exp *expecter[T]) ExpectAnyTimeout(timeout time.Duration, combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(or, &timeout, combiners)
}

// Expect adds a layer to this expecter with some number of combiners. The layer
// will be in 'AND' mode, which means it will become satisfied once ALL of the combiners
// becomes satisfied (versus with [ExpectAny] or [ExpectAnyTimeout], where only ONE of the
// combiners must become satisfied).
func (exp *expecter[T]) Expect(combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(and, nil, combiners)
}

// ExpectTimeout adds a layer to this expecter with some number of combiners. The layer
// will be in 'AND' mode, which means it will become satisfied once ALL of the combiners
// becomes satisfied (versus with [ExpectAny] or [ExpectAnyTimeout], where only ONE of the
// combiners must become satisfied).
//
// This layer will be created with a timeout. Once the layer is selected, the timeout will
// begin and messages delivered to the layer will be rejected once the timeout has elapsed.
func (exp *expecter[T]) ExpectTimeout(timeout time.Duration, combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(and, &timeout, combiners)
}

// Listen starts the expecter by launching a goroutinue
// which listens to the channel provided when creating the
// expecter, inside of a loop. If the channel the listener
// is reading from closes, the loop will close.
//
// Additionally, if all layers become satisfied, the loop will close
// as the expecter will consider itself fully satisfied.
//
// Unexpected messages will NOT cause the listener read loop to close. The message
// rejection will be tracked for later tracing output, but the read loop
// will continue unaffected.
//
// The expecter tracks this goroutine using a WaitGroup, this is used
// by AwaitSatisfied and AssertSatisfied to ensure the read-loop has
// closed. It is also used to detect if it has already closed.
//
// To forcefully close the listener read-loop, one can close the `closeChan` stored
// on the expecter. This is the mechanism that is used by the aforementioned
// AwaitSatisfied to force the listen loop to close after the timeout has been exceeded.
func (exp *expecter[T]) Listen() {
	if len(exp.expectLayers) == 0 {
		panic("no layers specified")
	}

	exp.wg.Add(1)
	go func() {
		defer exp.wg.Done()
		exp.currentLayerIndex = 0
		for {
			if exp.currentLayerIndex >= len(exp.expectLayers) {
				return
			}

			layer := exp.expectLayers[exp.currentLayerIndex]
			layer.Begin()

			select {
			case <-exp.closeChan:
				return
			case message, ok := <-exp.channel:
				if !ok {
					// channel closed
					return
				}

				if ok, trace := exp.shouldIgnoreMessage(message); ok {
					exp.results = append(exp.results, MessageResult[T]{
						Message:  message,
						LayerIdx: -1,
						Status:   Ignored,
						Trace:    trace,
					})

					continue
				}

				status := Rejected
				ok, trace := layer.TryMatch(message)
				if ok {
					status = Accepted
				}

				exp.results = append(exp.results, MessageResult[T]{
					Message:  message,
					LayerIdx: exp.currentLayerIndex,
					Status:   status,
					Trace:    trace,
				})

				if status == Accepted && layer.IsSatisfied() {
					exp.currentLayerIndex++
					continue
				}
			}
		}
	}()
}

// awaitFinished will wait for the expecter to finish, terminating
// it after the timeout specified. If the expecter finished without
// requiring termination, true is returned, else false.
func (exp *expecter[T]) awaitFinished(timeout time.Duration) bool {
	finished := make(chan struct{}, 1)
	go func() {
		exp.wg.Wait()
		finished <- struct{}{}
	}()

	terminated := false
	select {
	case <-time.NewTimer(timeout).C:
		terminated = true
		exp.closeChan <- struct{}{}
		<-finished
	case <-finished:
	}

	return !terminated
}

// AwaitSatisfied will wait (up to the timeout) for the expecter to see all layers
// specified as being satisfied. If the expecter does not become satisfied in time,
// it will be forcibly closed.
//
// The returns 'Errors' is a slice of all the errors found when looking through the
// state of the expecter. The errors will always wrap one of the following errors:
//   - [ErrUnsatisfiedExpecter]
//   - [ErrRejectedMessage]
//   - [ErrActiveLayerUnsatisfied]
func (exp *expecter[T]) AwaitSatisfied(timeout time.Duration) Errors {
	outErr := make([]error, 0)
	reportErr := func(err error) {
		outErr = append(outErr, err)
	}

	if !exp.awaitFinished(timeout) {
		reportErr(TerminatedError{timeout})
	}

	for idx, res := range exp.results {
		if res.Status == Rejected {
			reportErr(RejectionError[T]{MessageNum: idx, MessageResult: res})
		}
	}

	if exp.currentLayerIndex < len(exp.expectLayers) {
		currentLayer := exp.expectLayers[exp.currentLayerIndex]
		if currentLayer != nil && !currentLayer.IsSatisfied() {
			reportErr(UnsatisfiedError{exp.currentLayerIndex})
		}
	}

	return outErr
}

// TestingT is a minimal interface which mimics the standard
// [testing.T] struct. This is used in places that chanassert accepts
// a testing.T in order to allow unit testing of it's behaviour.
type TestingT interface {
	Errorf(format string, args ...any)
	Logf(format string, args ...any)
	Error(args ...any)
	Log(args ...any)
}

// AssertSatisfied ...
// expecter error: received messages which could not be matched (8 messages, across 2 layers).
// expecter error: layer #0: unexpected message: ”
// expecter error: layer #1: unexpected message: ”
// expecter error: failed to become satisfied: HINT: use .Debug() on your expecter to enable verbose message tracing.
func (exp *expecter[T]) AssertSatisfied(t TestingT, timeout time.Duration) {
	layers := make(map[int]struct{})
	rejections := 0
	errs := exp.AwaitSatisfied(timeout)
	for _, err := range errs {
		var rejectErr RejectionError[T]
		if errors.As(err, &rejectErr) {
			layers[rejectErr.MessageResult.LayerIdx] = struct{}{}
			rejections++
		}
	}

	if rejections > 0 {
		t.Errorf("expecter error: received messages which could not be matched (%d messages, across %d layers)", rejections, len(layers))
	}

	for _, e := range errs {
		stringBuilder := &strings.Builder{}
		stringBuilder.WriteString("expecter error: ")
		stringBuilder.WriteString(e.Error())

		if !exp.debug {
			var rejectErr RejectionError[T]
			if errors.As(e, &rejectErr) {
				stringBuilder.WriteString("\n")
				rejectErr.MessageResult.PrettyPrint(stringBuilder)
			}
		}

		t.Error(stringBuilder.String())
	}

	if len(errs) > 0 {
		if exp.debug {
			t.Errorf("expecter error: failed to become satisfied")

			stringBuilder := &strings.Builder{}
			stringBuilder.WriteString("EXPECTER: DEBUG enabled: trace of processed messages follow:\n")
			exp.FPrintTrace(stringBuilder)
			t.Log(stringBuilder.String())
		} else {
			t.Errorf("expecter error: failed to become satisfied: HINT: use .Debug() to enable verbose message tracing")
		}
	}
}

// Debug enables a full print out of the expecters FULL
// trace when any errors are reported by [AssertSatisfied]. This
// has no implications for other behaviour.
//
// If Debug is not called, then only traces for rejected
// messages are printed when using [AssertSatisfied].
func (exp *expecter[T]) Debug() Expecter[T] {
	exp.debug = true
	return exp
}

// PrintTrace prints a formatted representation
// of the expecter trace to stdout.
func (exp *expecter[T]) PrintTrace() {
	exp.FPrintTrace(os.Stdout)
}

// FPrintTrace prints a formatted representation
// of the expecter trace to the writer provided.
func (exp *expecter[T]) FPrintTrace(w io.Writer) {
	for _, msg := range exp.results {
		msg.PrettyPrint(w)
	}
}

// ProcessedMessages returns a messageResult for each of the messages
// processed by this expecter. Each messageResult contains the message itself,
// along with it's status (i.e. whether accepted, rejected, or ignored) as well
// as a trace which outlines the path the message took.
func (exp *expecter[T]) ProcessedMessages() []MessageResult[T] {
	return exp.results
}

// shouldIgnoreMessage checks if the given message matches
// any specified 'ignore' matcher. The boolean represents
// whether it should be ignored.
//
// If the message is ignored (bool true), then the associated trace
// message will contain information about which ignore matcher
// matched against the message.
//
// If the message was not ignored (bool false), then the trace will be empty.
func (exp *expecter[T]) shouldIgnoreMessage(message T) (bool, TraceMessage) {
	for idx, ignore := range exp.ignoreMatchers {
		if ignore.DoesMatch(message) {
			return true, newInfoTrace(fmt.Sprintf("Ignore matcher #%d ACCEPTED", idx))
		}
	}

	return false, newInfoTrace("")
}
