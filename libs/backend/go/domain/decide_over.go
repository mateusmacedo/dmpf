package domain

// DecideOver runs decide over a copy of *target and replaces *target with the
// copy only on acceptance (DEC-10, DEC-11). copyOf must copy in depth whatever
// decide can mutate: a shallow copy lets a refusal leak into *target (DEC-12).
func DecideOver[A, R any](target *A, copyOf func(*A) A, decide func(next *A) (Accepted[R], *Rejection)) (Accepted[R], *Rejection) {
	next := copyOf(target)
	accepted, rejection := decide(&next)
	if rejection != nil {
		return Accepted[R]{}, rejection
	}
	*target = next
	return accepted, nil
}

func Refuse[R any](code Code, message string, details ...Detail) (Accepted[R], *Rejection) {
	return Accepted[R]{}, Reject(code, message, details...)
}
