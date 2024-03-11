package chanassert

// AllOf accepts a list of matchers. The returned combiner will
// become satisfied once all the matchers have matched exactly one
// message. A matcher may only be matched against exactly once, to allow
// duplicate messages for the matchers, use 'AtLeastNOf'
func AllOf[T any](matchers ...Matcher[T]) *allOfCombiner[T] {
	return &allOfCombiner[T]{matchers: matchers, seen: make(map[int]struct{})}
}

type allOfCombiner[T any] struct {
	matchers  []Matcher[T]
	seen      map[int]struct{}
	satisfied bool
}

func (allOf *allOfCombiner[T]) DoesMatch(t T) bool {
	defer func() {
		allOf.satisfied = len(allOf.seen) == len(allOf.matchers)
	}()

	if allOf.satisfied {
		return false
	}

	for i, m := range allOf.matchers {
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
