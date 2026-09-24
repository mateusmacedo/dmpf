package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

const (
	ScramSHA256 = "SCRAM-SHA-256"
	ScramSHA512 = "SCRAM-SHA-512"
)

// SASL is the principal this client authenticates as. The broker ACL that
// binds the principal to the topics it may produce is the platform's.
type SASL struct {
	Mechanism string
	Username  string
	Password  string
}

func (s SASL) mechanism() (sasl.Mechanism, error) {
	auth := scram.Auth{User: s.Username, Pass: s.Password}
	switch s.Mechanism {
	case ScramSHA256:
		return auth.AsSha256Mechanism(), nil
	case ScramSHA512:
		return auth.AsSha512Mechanism(), nil
	default:
		return nil, ErrSASLMechanism
	}
}

// ClientAuth is what a process declares about its identity towards the
// broker: a SASL principal or a client certificate, and the authority of the
// broker's own certificate when it is private.
type ClientAuth struct {
	SASL     *SASL
	CertFile string
	KeyFile  string
	CAFile   string
}

// ReadClientAuth resolves the declaration from the environment, so the three
// processes that talk to Kafka read it the same way.
func ReadClientAuth(lookup func(string) string) ClientAuth {
	auth := ClientAuth{
		CertFile: lookup("DMPF_KAFKA_CLIENT_CERT_FILE"),
		KeyFile:  lookup("DMPF_KAFKA_CLIENT_KEY_FILE"),
		CAFile:   lookup("DMPF_KAFKA_CA_FILE"),
	}
	if mechanism := lookup("DMPF_KAFKA_SASL_MECHANISM"); mechanism != "" {
		auth.SASL = &SASL{Mechanism: mechanism, Username: lookup("DMPF_KAFKA_SASL_USERNAME"), Password: lookup("DMPF_KAFKA_SASL_PASSWORD")}
	}
	return auth
}

// apply puts the declaration on the configuration. SASL is kept without TLS
// only under the development opt-out, where the secret crosses in the clear.
func (a ClientAuth) apply(cfg *Config) error {
	cfg.SASL = a.SASL
	if cfg.TLS == nil {
		return nil
	}
	if a.CAFile != "" {
		authority, err := os.ReadFile(a.CAFile)
		if err != nil {
			return fmt.Errorf("kafka ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(authority) {
			return fmt.Errorf("kafka ca: %s holds no PEM certificate", a.CAFile)
		}
		cfg.TLS.RootCAs = pool
	}
	if a.CertFile != "" {
		pair, err := tls.LoadX509KeyPair(a.CertFile, a.KeyFile)
		if err != nil {
			return fmt.Errorf("kafka client certificate: %w", err)
		}
		cfg.TLS.Certificates = []tls.Certificate{pair}
	}
	return nil
}
