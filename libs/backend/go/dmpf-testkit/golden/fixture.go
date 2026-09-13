package golden

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

const FormatVersion = "1"

var (
	// ErrFormatVersion: reading a newer fixture with an older loader would report
	// a conformance nobody checked (FIX-09).
	ErrFormatVersion = errors.New("golden: unsupported format_version")
	// ErrNonStringScalar: a number read as a number loses precision in silence,
	// and with it the discriminating case the fixture exists for (FIX-07).
	ErrNonStringScalar = errors.New("golden: scalar is not a string")
)

// Fixture is the golden file as FIX-05 fixes it, every scalar a string. The
// JSON tags are the wire format shared with the TypeScript reader (FIX-02).
type Fixture struct {
	FormatVersion  string   `json:"format_version"`
	Identity       Identity `json:"identity"`
	Covers         Covers   `json:"covers"`
	Cases          []Case   `json:"cases"`
	Discriminators []Case   `json:"discriminators"`
}

type Identity struct {
	Fixture    string   `json:"fixture"`
	Contract   Contract `json:"contract"`
	Type       string   `json:"type"`
	DataSchema string   `json:"dataschema"`
}

type Contract struct {
	Package string `json:"package"`
	Message string `json:"message"`
}

type Covers struct {
	ProfileMajor  string `json:"profile_major"`
	ContractMajor string `json:"contract_major"`
}

type Case struct {
	Name            string            `json:"name"`
	Doc             string            `json:"doc"`
	Envelope        map[string]string `json:"envelope"`
	Payload         map[string]string `json:"payload"`
	PayloadBytesHex string            `json:"payload_bytes_hex"`
	PayloadHash     string            `json:"payload_hash"`
}

// Decode reads a fixture, refusing an unknown format_version before anything
// else and any scalar that is not a JSON string, naming its path.
func Decode(data []byte) (Fixture, error) {
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		return Fixture{}, fmt.Errorf("golden: %w", err)
	}
	version, ok := tree["format_version"].(string)
	if !ok {
		return Fixture{}, fmt.Errorf("%w: format_version", ErrNonStringScalar)
	}
	if version != FormatVersion {
		return Fixture{}, fmt.Errorf("%w: %q", ErrFormatVersion, version)
	}
	if err := StringScalars(data); err != nil {
		return Fixture{}, err
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return Fixture{}, fmt.Errorf("golden: %w", err)
	}
	return f, nil
}

// StringScalars is FIX-07 on its own: it reports the first scalar of a JSON
// document that is not a string, by path, so any string-typed fixture — wire
// or projection — can enforce the rule before decoding.
func StringScalars(data []byte) error {
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		return fmt.Errorf("golden: %w", err)
	}
	if path, bad := firstNonString(tree, ""); bad {
		return fmt.Errorf("%w: %s", ErrNonStringScalar, path)
	}
	return nil
}

func firstNonString(node any, path string) (string, bool) {
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if p, bad := firstNonString(v[k], join(path, k)); bad {
				return p, true
			}
		}
	case []any:
		for i, item := range v {
			if p, bad := firstNonString(item, path+"["+strconv.Itoa(i)+"]"); bad {
				return p, true
			}
		}
	case string:
	default:
		return path, true
	}
	return "", false
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// AllCases is the cases followed by the discriminators: every one must round
// trip as a consumer, only the cases as a producer (see Evaluate).
func (f Fixture) AllCases() []Case {
	return append(append([]Case(nil), f.Cases...), f.Discriminators...)
}

func (f Fixture) FindCase(name string) (Case, bool) {
	for _, c := range f.AllCases() {
		if c.Name == name {
			return c, true
		}
	}
	return Case{}, false
}
