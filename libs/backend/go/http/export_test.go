package http

var CategoryOf = categoryOf

func NotSent(err error) error { return &notSentError{err: err} }
