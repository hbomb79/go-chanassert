package chanassert

import (
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

func (nCombiner *nCombiner[T]) DoesMatch(message T) bool {
	if nCombiner.saturated {
		return false
	}

	defer nCombiner.updateSatisifed()
	defer nCombiner.updateSaturation()
	for i, m := range nCombiner.matchers {
		if m.DoesMatch(message) {
			if nCombiner.mode == modeEach {
				// If this matcher is saturated, then do not match against it anymore
				if c := nCombiner.counts[i]; c >= nCombiner.max {
					continue
				}
			}

			nCombiner.counts[i]++
			return true
		}
	}

	return false
}

func (nCombiner *nCombiner[T]) updateSaturation() {
	//exhaustive:enforce
	switch nCombiner.mode {
	case modeEach:
		// In 'each' mode, the combiner is saturated when ALL matchers have consumed their maximum
		if len(nCombiner.counts) != len(nCombiner.matchers) {
			nCombiner.saturated = false
			return
		}

		for _, c := range nCombiner.counts {
			if c < nCombiner.max {
				nCombiner.saturated = false
				return
			}
		}

		// All matchers are at max. Saturated!
		nCombiner.saturated = true
	case modeAny:
		// In 'any' mode, the combiner is saturated when any ONE matcher has consumed the maximum
		for _, c := range nCombiner.counts {
			if c >= nCombiner.max {
				nCombiner.saturated = true
				return
			}
		}

		nCombiner.saturated = false
	case modeSum:
		// In 'sum' mode, the combiner is saturated when the sum of matched messages has reached the maximum
		nCombiner.saturated = nCombiner.sumMatches() >= nCombiner.max
	}
}

func (nCombiner *nCombiner[T]) updateSatisifed() {
	//exhaustive:enforce
	switch nCombiner.mode {
	case modeEach:
		if len(nCombiner.counts) != len(nCombiner.matchers) {
			nCombiner.satisfied = false
			return
		}

		for _, c := range nCombiner.counts {
			if c < nCombiner.min || c > nCombiner.max {
				nCombiner.satisfied = false
				return
			}
		}

		// All matchers are between min and max. Satisfied!
		nCombiner.satisfied = true
		return
	case modeAny:
		for _, c := range nCombiner.counts {
			if c >= nCombiner.min && c <= nCombiner.max {
				// At least one of the matchers are between min and max. Satisfied!
				nCombiner.satisfied = true
				return
			}
		}

		// None of the matchers are between min and max. NOT Satisfied!
		nCombiner.satisfied = false
		return
	case modeSum:
		count := nCombiner.sumMatches()
		nCombiner.satisfied = count >= nCombiner.min && count <= nCombiner.max
		return
	}
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
