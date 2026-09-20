module github.com/mateusmacedo/dmpf/apps/backend/bff

go 1.26.6

require (
	github.com/jackc/pgx/v5 v5.10.0
	github.com/mateusmacedo/dmpf/libs/backend/go/contracts v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/grpc v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/http v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/observability v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/testkit v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/transport v0.1.0
	github.com/twmb/franz-go v1.21.6
	github.com/twmb/franz-go/pkg/kadm v1.18.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/sdk v1.46.0
	go.opentelemetry.io/otel/sdk/metric v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v0.1.0
)
