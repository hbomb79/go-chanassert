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
	"testing"
	"time"
)

var (
	ErrUnsatisfiedExpecter    = errors.New("expecter did not become satisfied within timeout")
	ErrRejectedMessage        = errors.New("expecter received message which was rejected")
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

	Begin()
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

	AssertSatisfied(t *testing.T, timeout time.Duration)
	AwaitSatisfied(timeout time.Duration) Errors

	PrintTrace()
	FPrintTrace(w io.Writer)
	ProcessedMessages() []messageResult[T]

	Listen()
}

type expecter[T any] struct {
	channel        chan T
	ignoreMatchers []Matcher[T]
	currentLayer   int
	expectLayers   []Layer[T]
	wg             *sync.WaitGroup
	closeChan      chan struct{}
	results        []messageResult[T]
}

func NewChannelExpecter[T any](channel chan T) *expecter[T] {
	return &expecter[T]{
		channel:        channel,
		currentLayer:   0,
		ignoreMatchers: make([]Matcher[T], 0),
		expectLayers:   make([]Layer[T], 0),
		closeChan:      make(chan struct{}, 1),
		wg:             &sync.WaitGroup{},
		results:        make([]messageResult[T], 0),
	}
}

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

func (exp *expecter[T]) ExpectAny(combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(or, nil, combiners)
}

func (exp *expecter[T]) ExpectAnyTimeout(timeout time.Duration, combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(or, &timeout, combiners)
}

func (exp *expecter[T]) Expect(combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(and, nil, combiners)
}

func (exp *expecter[T]) ExpectTimeout(timeout time.Duration, combiners ...Combiner[T]) Expecter[T] {
	return exp.addLayer(and, &timeout, combiners)
}

// AwaitSatisfied will wait (up to the timeout) for the expecter to see all layers
// specified as being satisfied. If the expecter does not become satisfied in time,
// it will be forcibly closed.
//
// The returns 'Errors' is a slice of all the errors found when looking through the
// state of the expecter. The errors are:
//   - UnsatisfiedExpecterErr, which indicates that the expecter did not 'finish' on
//     it's own before the timeout provided to this function,
//   - RejectedMessageErr, which indicates a message was received by the expecter
//     but could not be matched against the active layer,
//   - ActiveLayerUnsatisfiedErr, which indicates that the active layer at the time
//     of the expecter finishing was not satisfied. In the absence of an
//     UnsatisfiedExpecterErr, this indicates the message channel closed before the expecter
//     witnessed enough messages to satisfy it's expectations.
func (exp *expecter[T]) AwaitSatisfied(timeout time.Duration) Errors {
	outErr := make([]error, 0)
	reportErr := func(err error) {
		outErr = append(outErr, err)
	}

	// Wait for the listen loop to close, up to timeout. Otherwise, close it manually
	finished := make(chan struct{}, 1)
	go func() {
		exp.wg.Wait()
		finished <- struct{}{}
	}()

	select {
	case <-time.NewTimer(timeout).C:
		reportErr(ErrUnsatisfiedExpecter)
		exp.closeChan <- struct{}{}
		<-finished
	case <-finished:
	}

	for idx, res := range exp.results {
		if res.Status == rejected {
			reportErr(fmt.Errorf("message #%d (%+v) REJECTED: %w", idx, res.Message, ErrRejectedMessage))
		}
	}

	if exp.currentLayer < len(exp.expectLayers) {
		maybeLayer := exp.expectLayers[exp.currentLayer]
		if maybeLayer != nil {
			if !maybeLayer.IsSatisfied() {
				reportErr(fmt.Errorf("layer #%d: %w", exp.currentLayer, ErrActiveLayerUnsatisfied))
			}
		}
	}

	return outErr
}

func (exp *expecter[T]) PrintTrace() {
	exp.FPrintTrace(os.Stdout)
}

func (exp *expecter[T]) FPrintTrace(w io.Writer) {
	fmt.Fprint(w, "EXPECTER: trace of processed messages follow:\n")
	for _, msg := range exp.results {
		msg.PrettyPrint(w)
	}
}

// ProcessedMessages returns a messageResult for each of the messages
// processed by this expecter. Each messageResult contains the message itself,
// along with it's status (i.e. whether accepted, rejected, or ignored) as well
// as a trace which outlines the path the message took.
func (exp *expecter[T]) ProcessedMessages() []messageResult[T] {
	return exp.results
}

func (exp *expecter[T]) AssertSatisfied(t *testing.T, timeout time.Duration) {
	errors := exp.AwaitSatisfied(timeout)
	for _, e := range errors {
		t.Errorf("expecter error: %s", e)
	}

	if len(errors) > 0 {
		t.Errorf("satisified assertion failed: expecter encountered errors (%d)", len(errors))

		stringBuilder := &strings.Builder{}
		exp.FPrintTrace(stringBuilder)
		t.Log(stringBuilder)
	}
}

func (exp *expecter[T]) shouldIgnoreMessage(message T) (bool, TraceMessage) {
	for idx, ignore := range exp.ignoreMatchers {
		if ignore.DoesMatch(message) {
			return true, newInfoTrace(fmt.Sprintf("Ignore matcher #%d ACCEPTED", idx))
		}
	}

	return false, newInfoTrace("")
}

func (exp *expecter[T]) Listen() {
	if len(exp.expectLayers) == 0 {
		panic("no layers specified")
	}

	exp.wg.Add(1)
	go func() {
		defer exp.wg.Done()
		exp.currentLayer = 0
		for {
			if exp.currentLayer >= len(exp.expectLayers) {
				return
			}

			layer := exp.expectLayers[exp.currentLayer]
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
					exp.results = append(exp.results, messageResult[T]{
						Message: message,
						Status:  ignored,
						Trace:   trace,
					})

					continue
				}

				status := rejected
				ok, trace := layer.TryMatch(message)
				if ok {
					status = accepted
				}

				exp.results = append(exp.results, messageResult[T]{
					Message: message,
					Status:  status,
					Trace:   trace,
				})

				if status == accepted && layer.IsSatisfied() {
					exp.currentLayer++
					continue
				}
			}
		}
	}()
}
