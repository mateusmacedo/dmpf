package redact

import "strings"

var secretWords = map[string]bool{
	"password": true, "passwd": true, "secret": true, "token": true, "credential": true,
	"credentials": true, "authorization": true, "dsn": true, "apikey": true, "cookie": true,
}

func IsSecret(key string) bool {
	words := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool { return r == '.' || r == '_' || r == '-' })
	for i, word := range words {
		if secretWords[word] {
			return true
		}
		if word == "key" && i > 0 && (words[i-1] == "private" || words[i-1] == "api") {
			return true
		}
	}
	return false
}
