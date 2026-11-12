module github.com/mateusmacedo/dmpf/libs/backend/go/s3

go 1.26.11

require (
	github.com/aws/aws-sdk-go-v2 v1.46.0
	github.com/aws/aws-sdk-go-v2/credentials v1.20.3
	github.com/aws/aws-sdk-go-v2/service/s3 v1.112.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.49.0
	github.com/aws/smithy-go v1.28.1
	github.com/mateusmacedo/dmpf/libs/backend/go/observability v1.0.0-rc.9
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v1.0.0-rc.9
	github.com/mateusmacedo/dmpf/libs/backend/go/transport v1.0.0-rc.9
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/log v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
)

require (
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.19.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.42.0 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/contracts v1.0.0-rc.9 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/memory v1.0.0-rc.9 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.33.3
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/kms v1.59.0
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v1.0.0-rc.9 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/testkit v1.0.0-rc.9
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/bridges/otelslog v0.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
