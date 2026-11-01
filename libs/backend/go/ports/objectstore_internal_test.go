package ports

// WHY: positional literals stop compiling when a field is added, so a bypass
// flag slipped into a request, a retention or a hold cannot pass unreviewed (DAT-11).
var (
	_ = PutRequest{ObjectKey(""), nil, "", "", nil}
	_ = Retention{RetentionMode(""), Instant(0)}
	_ = LegalHold{false, ""}
	_ = ObjectRef{ObjectKey(""), VersionID("")}
)
