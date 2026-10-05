package postgres_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type widgetID string

type widget struct {
	ID    widgetID
	Size  int
	Parts []string
}

type widgetState struct {
	Size  int      `json:"size"`
	Parts []string `json:"parts"`
}

var widgetTable = postgres.SnapshotTable("widgets", "widget_id",
	func(w widget) widgetState {
		return widgetState{Size: w.Size, Parts: append(make([]string, 0, len(w.Parts)), w.Parts...)}
	},
	func(s widgetState) widget { return widget{Size: s.Size, Parts: s.Parts} },
	func(w widget, id widgetID) widget { w.ID = id; return w },
)

func scanBytes(raw []byte) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*[]byte) = raw
		return nil
	}
}

func TestSnapshotTableKeepsTheWholeStateInTheSnapshotColumn(t *testing.T) {
	if widgetTable.Name != "widgets" || widgetTable.IDColumn != "widget_id" {
		t.Fatalf("table = %s(%s), want widgets(widget_id)", widgetTable.Name, widgetTable.IDColumn)
	}
	if !slices.Equal(widgetTable.Columns, []string{"snapshot"}) {
		t.Fatalf("Columns = %q, want [snapshot]", widgetTable.Columns)
	}
}

func TestSnapshotTableEncodesTheBytesOfTheProviderState(t *testing.T) {
	values, err := widgetTable.Encode(widget{ID: "w-1", Size: 3})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	want, _ := json.Marshal(widgetState{Size: 3, Parts: []string{}})
	if len(values) != 1 || string(values[0].([]byte)) != string(want) {
		t.Fatalf("Encode = %q, want one column with %s", values, want)
	}
}

func TestSnapshotTableDecodesAndRestoresTheID(t *testing.T) {
	values, err := widgetTable.Encode(widget{ID: "w-1", Size: 3, Parts: []string{"a", "b"}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := widgetTable.Decode(scanBytes(values[0].([]byte)))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got := widgetTable.WithID(decoded, "w-1")

	if got.ID != "w-1" || got.Size != 3 || !slices.Equal(got.Parts, []string{"a", "b"}) {
		t.Fatalf("round trip = %+v, want {w-1 3 [a b]}", got)
	}
}

func TestSnapshotTableRejectsAColumnThatIsNotTheState(t *testing.T) {
	if _, err := widgetTable.Decode(scanBytes([]byte("not json"))); err == nil {
		t.Fatal("Decode accepted bytes that are not the JSON state")
	}
}
