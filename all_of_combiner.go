package chanassert

// allOfCombiner becomes satisfied after seeing any value which
// matches a single matcher, and will not accept any additional values.
type allOfCombiner[T any] struct {
	matchers  []Matcher[T]
	seen      map[int]struct{}
	satisfied bool
}

func AllOf[T any](matchers ...Matcher[T]) *allOfCombiner[T] {
	return &allOfCombiner[T]{matchers: matchers, seen: make(map[int]struct{})}
}

func (allOf *allOfCombiner[T]) DoesMatch(t T) bool {
	defer func() {
		if len(allOf.seen) == len(allOf.matchers) {
			allOf.satisfied = true
		}
	}()

	if allOf.satisfied {
		return false
	}

	for i, m := range allOf.matchers {
		// Do not allow multiple messages to match the same matcher multiple times
		if _, seen := allOf.seen[i]; seen {
			continue
		}

		if m.DoesMatch(t) {
			allOf.seen[i] = struct{}{}
			return true
		}
	}

	return false
}

func (allOf *allOfCombiner[T]) IsSatisfied() bool {
	return allOf.satisfied
}
