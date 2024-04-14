package chanassert

import (
	"fmt"
	"math"
)

// AllOf accepts a list of matchers. The returned combiner will
// become satisfied once all the matchers have matched exactly one
// message. A matcher may only be matched against exactly once, to allow
// duplicate messages for each of the matchers, you can use 'AtLeastNOfEach'.
func AllOf[T any](matchers ...Matcher[T]) *nCombiner[T] {
	return ExactlyNOfEach(1, matchers...)
}

// OneOf accepts a list of matchers. The returned combiner will become
// satisfied once ANY of the matchers match a message. Once satisfied, this combiner
// will no longer match new messages.
func OneOf[T any](matchers ...Matcher[T]) *nCombiner[T] {
	return ExactlyNOf(1, matchers...)
}

// AtLeastNOfEach accepts an number, n, and a list of matchers, and returns
// a combiner which will become satisfied once EACH of the matchers provided
// have successfully matched an incoming message AT LEAST N times.
func AtLeastNOfEach[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeEach, matchers: matchers, min: n, max: math.MaxInt, counts: make(map[int]int)}
}

// BetweenNOfEach accepts two numbers, min and max, and a list of matchers. The returned
// combiner will become satisfied once EACH of the matchers provided
// have successfully matched an incoming message AT LEAST MIN times, and NO MORE THAN MAX times.
func BetweenNOfEach[T any](min int, max int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeEach, matchers: matchers, min: min, max: max, counts: make(map[int]int)}
}

// ExactlyNOfEach accepts a number, n, and a list of matchers. The returned
// combiner will be satisfied when EACH of the matchers have matched incoming messages
// EXACTLY N TIMES.
func ExactlyNOfEach[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeEach, matchers: matchers, min: n, max: n, counts: make(map[int]int)}
}

// AtLeastNOfAny accepts an number, n, and a list of matchers, and returns
// a combiner which will become satisfied once ANY of the matchers provided.
func AtLeastNOfAny[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeAny, matchers: matchers, min: n, max: math.MaxInt, counts: make(map[int]int)}
}

// BetweenNOfAny accepts two numbers, min and max, and a list of matchers. The returned
// combiner will become satisfied once ANY of the matchers provided
// have successfully matched an incoming message AT LEAST MIN times, and NO MORE THAN MAX times.
func BetweenNOfAny[T any](min int, max int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeAny, matchers: matchers, min: min, max: max, counts: make(map[int]int)}
}

// ExactlyNOfAny accepts a number, n, and a list of matchers. The returned
// combiner will be satisfied when ANY of the matchers have matched incoming messages
// EXACTLY N TIMES.
func ExactlyNOfAny[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeAny, matchers: matchers, min: n, max: n, counts: make(map[int]int)}
}

// AtLeastNOf accepts an number, n, and a list of matchers, and returns
// a combiner which will become satisfied once the total sum of all messages matched against
// any combination of the matchers is AT LEAST N.
func AtLeastNOf[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeSum, matchers: matchers, min: n, max: math.MaxInt, counts: make(map[int]int)}
}

// BetweenNOf accepts two numbers, min and max, and a list of matchers, and returns
// a combiner which will become satisfied once the total SUM of all messages matched against
// any combination of the provided matchers is BETWEEN MIN and MAX.
func BetweenNOf[T any](min int, max int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeSum, matchers: matchers, min: min, max: max, counts: make(map[int]int)}
}

// ExactlyNOf accepts an number, n, and a list of matchers, and returns
// a combiner which will become satisfied once the total SUM of all messages matched against
// any combination of the provided matchers is EXACTLY N.
func ExactlyNOf[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{mode: modeSum, matchers: matchers, min: n, max: n, counts: make(map[int]int)}
}

type mode int

const (
	modeEach mode = iota
	modeAny
	modeSum
)

func (m mode) String() string {
	return []string{"EACH", "ANY", "SUM"}[m]
}

type nCombiner[T any] struct {
	matchers []Matcher[T]
	min      int
	max      int
	counts   map[int]int
	mode     mode

	// satisfied indicates whether this matcher can be considered 'done' by
	// the layer which holds it. The matcher *may* be able to accept more messages (assuming
	// it's not also saturated), however it doesn't *need* to do so.
	satisfied bool

	// saturated indicates whether the combiner is able to match against
	// more messages. Depending on the mode, this value is set under different circumstances.
	// Once saturated, any call to DoesMatch will return false.
	saturated bool
}

func (nCombiner *nCombiner[T]) tryMatch(message T) (bool, TraceMessage) {
	if nCombiner.saturated {
		return false, newInfoTrace("Combiner is fully saturated, accepting no further messages")
	}

	attempts := make([]TraceMessage, 0)
	for i, m := range nCombiner.matchers {
		if m.DoesMatch(message) {
			if nCombiner.mode == modeEach {
				// If this matcher is saturated, then do not match against it anymore
				if c := nCombiner.counts[i]; c >= nCombiner.max {
					attempts = append(attempts, newInfoTrace(fmt.Sprintf("Matcher #%d REJECT: matcher has already matched maximum allowed messages", i)))
					continue
				}
			}

			nCombiner.counts[i]++
			attempts = append(attempts, newInfoTrace(fmt.Sprintf("Matcher #%d ACCEPT", i)))
			return true, newInfoTrace(fmt.Sprintf("Combiner matched on matcher #%d", i), attempts...)
		} else {
			attempts = append(attempts, newInfoTrace(fmt.Sprintf("Matcher #%d REJECT: no match", i)))
		}
	}

	return false, newInfoTrace("Combiner failed match message", attempts...)
}

// TryMatch attempts to match the given message against
// the matchers contained within the combiner. If a match
// is made, the returned bool will be true. Additionally to this bool, a
// TraceMessage is returned alongside (regardless of the match being successful or not)
// which contains information about the messages handling.
func (nCombiner *nCombiner[T]) TryMatch(message T) (bool, TraceMessage) {
	ok, trace := nCombiner.tryMatch(message)

	modeTrace := newInfoTrace(fmt.Sprintf("%s mode with minimum of %d and maximum of %d", nCombiner.mode, nCombiner.min, nCombiner.max))
	satisfiedTrace := nCombiner.updateSatisifed()
	saturatedTrace := nCombiner.updateSaturation()

	status := newInfoTrace("Combiner status", modeTrace, satisfiedTrace, saturatedTrace, nCombiner.matcherCountsTrace())

	trace.Nested = append(trace.Nested, status)
	return ok, trace
}

func (nCombiner *nCombiner[T]) updateSaturation() TraceMessage {
	generateTrace := func(isSaturated bool, reason string) TraceMessage {
		if isSaturated {
			return newInfoTrace("Saturated", newDebugTrace(reason))
		}

		return newInfoTrace("NOT saturated", newDebugTrace(reason))
	}

	//exhaustive:enforce
	switch nCombiner.mode {
	case modeEach:
		// In 'each' mode, the combiner is saturated when ALL matchers have consumed their maximum
		if len(nCombiner.counts) != len(nCombiner.matchers) {
			nCombiner.saturated = false

			missing := make(idxList, 0)
			for idx := range nCombiner.matchers {
				if _, ok := nCombiner.counts[idx]; !ok {
					missing = append(missing, idx)
				}
			}

			return generateTrace(false, fmt.Sprintf("EACH matcher needs to match at least %d messages, but matchers %v have yet to match any messages", nCombiner.max, missing))
		}

		notSaturated := make(idxList, 0)
		for idx, c := range nCombiner.counts {
			if c < nCombiner.max {
				notSaturated = append(notSaturated, idx)
			}
		}

		nCombiner.saturated = len(notSaturated) == 0
		if nCombiner.saturated {
			return generateTrace(true, fmt.Sprintf("EACH matcher has matched maximum allowed messages (%d)", nCombiner.max))
		}

		return generateTrace(false, fmt.Sprintf("EACH matcher needs to match at least %d messages, but matchers %v have not", nCombiner.max, notSaturated))
	case modeAny:
		// In 'any' mode, the combiner is saturated when any ONE matcher has consumed the maximum
		for idx, c := range nCombiner.counts {
			if c >= nCombiner.max {
				nCombiner.saturated = true

				return generateTrace(true, fmt.Sprintf("Matcher #%d has matched against maximum messages (%d)", idx, nCombiner.max))
			}
		}

		nCombiner.saturated = false
		return generateTrace(false, fmt.Sprintf("ANY matcher needs to match %d messages, but none have", nCombiner.max))
	case modeSum:
		// In 'sum' mode, the combiner is saturated when the sum of matched messages has reached the maximum
		sum := nCombiner.sumMatches()
		nCombiner.saturated = sum >= nCombiner.max
		if nCombiner.saturated {
			return generateTrace(true, fmt.Sprintf("SUM of all matched messages (%d) has met maximum (%d) messages", sum, nCombiner.max))
		}

		return generateTrace(false, fmt.Sprintf("SUM of all matched messages (%d) must be at least %d", sum, nCombiner.max))
	}

	panic("unreachable")
}

func (nCombiner *nCombiner[T]) updateSatisifed() TraceMessage {
	generateTrace := func(isSatisfied bool, reason string) TraceMessage {
		if isSatisfied {
			return newInfoTrace("Satisfied", newDebugTrace(reason))
		} else {
			return newInfoTrace("NOT satisfied", newDebugTrace(reason))
		}
	}

	//exhaustive:enforce
	switch nCombiner.mode {
	case modeEach:
		if len(nCombiner.counts) != len(nCombiner.matchers) {
			nCombiner.satisfied = false

			missing := make(idxList, 0)
			for idx := range nCombiner.matchers {
				if _, ok := nCombiner.counts[idx]; !ok {
					missing = append(missing, idx)
				}
			}

			return generateTrace(false, fmt.Sprintf("EACH matcher needs to match at least %d messages, but matchers %v have yet to match any messages", nCombiner.min, missing))
		}

		notSatisfied := make(idxList, 0)
		for idx, c := range nCombiner.counts {
			if c < nCombiner.min || c > nCombiner.max {
				notSatisfied = append(notSatisfied, idx)
			}
		}

		nCombiner.satisfied = len(notSatisfied) == 0
		if nCombiner.satisfied {
			return generateTrace(true, fmt.Sprintf("EACH matcher has matched at least %d messages", nCombiner.min))
		}

		return generateTrace(false, fmt.Sprintf("ALL matchers needs to match at least %d messages, but matchers %v have not", nCombiner.min, notSatisfied))
	case modeAny:
		for idx, c := range nCombiner.counts {
			if c >= nCombiner.min && c <= nCombiner.max {
				// At least one of the matchers are between min and max. Satisfied!
				nCombiner.satisfied = true
				return generateTrace(true, fmt.Sprintf("Matcher #%d has matched against minimum messages (%d)", idx, nCombiner.min))
			}
		}

		// None of the matchers are between min and max. NOT Satisfied!
		nCombiner.satisfied = false
		return generateTrace(false, fmt.Sprintf("ANY matcher needs to match at least %d messages, but none have", nCombiner.min))
	case modeSum:
		count := nCombiner.sumMatches()
		nCombiner.satisfied = count >= nCombiner.min && count <= nCombiner.max

		if nCombiner.satisfied {
			return generateTrace(true, fmt.Sprintf("SUM of all matched messages (%d) has met minimum (%d) messages", count, nCombiner.min))
		}

		return generateTrace(false, fmt.Sprintf("SUM of all matched messages (%d) must meet %d messages", count, nCombiner.min))
	}

	panic("unreachable")
}

func (nCombiner *nCombiner[T]) matcherCountsTrace() TraceMessage {
	details := make([]TraceMessage, 0, len(nCombiner.matchers))
	for k := range nCombiner.matchers {
		if count, ok := nCombiner.counts[k]; ok {
			details = append(details, newInfoTrace(fmt.Sprintf("Matcher #%d => %d message(s)", k, count)))
		} else {
			details = append(details, newInfoTrace(fmt.Sprintf("Matcher #%d => 0 messages", k)))
		}
	}

	return newDebugTrace("Matcher counts", details...)
}

func (nCombiner *nCombiner[T]) sumMatches() int {
	count := 0
	for _, matches := range nCombiner.counts {
		count += matches
	}

	return count
}

func (nCombiner *nCombiner[T]) IsSatisfied() bool {
	return nCombiner.satisfied
}
