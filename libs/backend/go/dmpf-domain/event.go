package dmpfdomain

// DomainEvent is a fact the aggregate produced, named in the past tense and
// free of version, transport or wire vocabulary (FND-03 §5.3, MSG-N01..N03).
type DomainEvent interface {
	EventName() string
}
