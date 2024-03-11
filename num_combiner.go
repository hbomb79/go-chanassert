package chanassert

import "math"

// AtLeastNOf accepts an number, n, and a list of matchers, and returns
// a combiner which will become satisfied once ALL the matchers provided
// have successfully matched an incoming message AT LEAST N times.
// This matcher is greedy, and will continue to match against messages while satisfied.
func AtLeastNOf[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{matchers: matchers, min: n, max: math.MaxInt, counts: make(map[int]int)}
}

// BetweenNOf accepts two numbers, min and max, and a list of matchers. The returned
// combiner will become satisfied once ALL the matchers provided
// have successfully matched an incoming message AT LEAST MIN times, and NO MORE THAN MAX times.
// This matcher is greedy, and will continue to match against messages while satisfied.
func BetweenNOf[T any](min int, max int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{matchers: matchers, min: min, max: max, counts: make(map[int]int)}
}

// ExactlyNOf accepts a number, n, and a list of matchers. The returned
// combiner will be satisfied when all the matchers have matched incoming messages
// EXACTLY N TIMES.
// This matcher is greedy, and will continue to match against messages while satisfied.
func ExactlyNOf[T any](n int, matchers ...Matcher[T]) *nCombiner[T] {
	return &nCombiner[T]{matchers: matchers, min: n, max: n, counts: make(map[int]int)}
}

type nCombiner[T any] struct {
	matchers []Matcher[T]
	min      int
	max      int
	counts   map[int]int
}

func (nCombiner *nCombiner[T]) DoesMatch(t T) bool {
	for i, m := range nCombiner.matchers {
		if m.DoesMatch(t) {
			nCombiner.counts[i]++
			return true
		}
	}

	return false
}

func (nCombiner *nCombiner[T]) IsSatisfied() bool {
	if len(nCombiner.counts) < len(nCombiner.matchers) {
		return false
	}

	for _, c := range nCombiner.counts {
		if c < nCombiner.min || c > nCombiner.max {
			return false
		}
	}

	return true
}
