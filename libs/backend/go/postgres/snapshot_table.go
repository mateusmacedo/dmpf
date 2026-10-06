package postgres

import "encoding/json"

// SnapshotTable maps an aggregate whose whole state lives in the jsonb snapshot
// column (ADR-053): J is the provider's private state, whose JSON tags fix the
// stored layout, and the id lives only in idColumn, restored by withID.
func SnapshotTable[ID ~string, S, J any](
	name, idColumn string,
	to func(S) J,
	from func(J) S,
	withID func(S, ID) S,
) Table[ID, S] {
	return Table[ID, S]{
		Name:     name,
		IDColumn: idColumn,
		Columns:  []string{"snapshot"},

		Encode: func(s S) ([]any, error) {
			raw, err := json.Marshal(to(s))
			if err != nil {
				return nil, err
			}
			return []any{raw}, nil
		},

		Decode: func(scan func(dest ...any) error) (S, error) {
			var zero S
			var raw []byte
			if err := scan(&raw); err != nil {
				return zero, err
			}
			var state J
			if err := json.Unmarshal(raw, &state); err != nil {
				return zero, err
			}
			return from(state), nil
		},

		WithID: withID,
	}
}
