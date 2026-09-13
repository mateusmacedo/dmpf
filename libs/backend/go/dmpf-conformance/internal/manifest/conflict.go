package manifest

// ConflictPairs returns field pairs where legacy and new representation diverge.
// Empty when there is no conflict or when one side is absent.
func (x Exception) ConflictPairs() []string {
	var out []string
	if x.PresentObject {
		if x.Unit != "" && x.Object.PresentUnit && x.Object.Unit != "" && x.Unit != x.Object.Unit {
			out = append(out, "unit vs object.unit")
		}
		if x.Dependency != "" && x.Object.PresentIdentity && x.Object.Identity != "" && x.Dependency != x.Object.Identity {
			out = append(out, "dependency vs object.identity")
		}
	}
	if x.PresentJustification && x.Reason != "" && x.Justification != "" && x.Reason != x.Justification {
		out = append(out, "reason vs justification")
	}
	return out
}
