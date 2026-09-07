package tb

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/domainkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/golden"
)

// ProjectionFormatVersion is the format the projection fixtures carry; an
// unknown one fails the codec (FIX-09).
const ProjectionFormatVersion = "1"

// ErrProjectionFormatVersion is what DecodeProjection returns for a version it
// does not know.
var ErrProjectionFormatVersion = errors.New("tb: unsupported projection format_version")

// ProjectionFixture is the observable-outcome fixture of ORA-30: input (state
// before and command) and the expected projection, no wire bytes and no hash.
// The codec lives here and not in domainkit because encoding/json is
// a wire.codec capability, which the domain block lacks.
type ProjectionFixture struct {
	FormatVersion string             `json:"format_version"`
	Identity      ProjectionIdentity `json:"identity"`
	Cases         []ProjectionCase   `json:"cases"`
}

type ProjectionIdentity struct {
	Context   string `json:"context"`
	Aggregate string `json:"aggregate"`
}

type ProjectionCase struct {
	Name        string             `json:"name"`
	Doc         string             `json:"doc"`
	StateBefore domainkit.Fields   `json:"state_before"`
	Command     domainkit.Fields   `json:"command"`
	Expected    ExpectedProjection `json:"expected"`
}

type ExpectedProjection struct {
	Branch     domainkit.Branch  `json:"branch"`
	Response   domainkit.Fields  `json:"response,omitempty"`
	Rejection  ExpectedRejection `json:"rejection,omitempty"`
	Events     []ExpectedEvent   `json:"events"`
	StateAfter domainkit.Fields  `json:"state_after,omitempty"`
}

type ExpectedRejection struct {
	Code    string           `json:"code,omitempty"`
	Message string           `json:"message,omitempty"`
	Details domainkit.Fields `json:"details,omitempty"`
}

type ExpectedEvent struct {
	Name   string           `json:"name"`
	Fields domainkit.Fields `json:"fields"`
}

// Projection is the expected outcome as domainkit compares it.
func (e ExpectedProjection) Projection() domainkit.Projection {
	p := domainkit.Projection{
		Branch:     e.Branch,
		Response:   e.Response,
		Rejection:  domainkit.Rejection{Code: e.Rejection.Code, Message: e.Rejection.Message, Details: e.Rejection.Details},
		StateAfter: e.StateAfter,
		Events:     make([]domainkit.Event, 0, len(e.Events)),
	}
	for _, ev := range e.Events {
		p.Events = append(p.Events, domainkit.Event{Name: ev.Name, Fields: ev.Fields})
	}
	return p
}

// DecodeProjection reads a projection fixture with the same two refusals as
// golden.Decode: unknown format_version and any non-string scalar.
func DecodeProjection(data []byte) (ProjectionFixture, error) {
	var head struct {
		FormatVersion any `json:"format_version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return ProjectionFixture{}, fmt.Errorf("tb: %w", err)
	}
	version, ok := head.FormatVersion.(string)
	if !ok {
		return ProjectionFixture{}, fmt.Errorf("%w: format_version", golden.ErrNonStringScalar)
	}
	if version != ProjectionFormatVersion {
		return ProjectionFixture{}, fmt.Errorf("%w: %q", ErrProjectionFormatVersion, version)
	}
	if err := golden.StringScalars(data); err != nil {
		return ProjectionFixture{}, err
	}
	var f ProjectionFixture
	if err := json.Unmarshal(data, &f); err != nil {
		return ProjectionFixture{}, fmt.Errorf("tb: %w", err)
	}
	return f, nil
}

// LoadProjection reads and decodes a projection fixture by its path relative
// to the repository root.
func LoadProjection(t testing.TB, rel string) ProjectionFixture {
	t.Helper()
	f, err := DecodeProjection(ReadFixture(t, rel))
	if err != nil {
		t.Fatalf("tb.LoadProjection %s: %v", rel, err)
	}
	return f
}
