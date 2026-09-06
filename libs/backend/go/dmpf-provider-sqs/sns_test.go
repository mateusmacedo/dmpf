package dmpfsqs_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	dmpfsqs "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-sqs"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

const subARN = topicARN + ":sub-1"

func snsOnlyConfig() dmpfsqs.Config {
	cfg := validConfig()
	cfg.Catalog = channel.Catalog{"orders-fanout": snsChannel()}
	return cfg
}

func TestNewSNSPublisherDiscoversTheSubscriptionsAndRequiresRawDelivery(t *testing.T) {
	api := dmpfsqs.NewFakeSNS()
	api.Subscribe(topicARN, subARN, "sqs", map[string]string{"RawMessageDelivery": "true"})
	api.Subscribe(topicARN, topicARN+":sub-wrapped", "sqs", map[string]string{"RawMessageDelivery": "false"})

	_, err := dmpfsqs.NewSNSPublisher(context.Background(), snsOnlyConfig(), api)
	if !errors.Is(err, dmpfsqs.ErrRawDeliveryRequired) {
		t.Fatalf("NewSNSPublisher() = %v, want ErrRawDeliveryRequired: the broker's own list is what is verified (SQS-02)", err)
	}
	if !strings.Contains(err.Error(), "sub-wrapped") {
		t.Fatalf("error %v does not name the offending subscription", err)
	}
}

func TestNewSNSPublisherIgnoresNonSQSSubscriptionsButRequiresOne(t *testing.T) {
	api := dmpfsqs.NewFakeSNS()
	api.Subscribe(topicARN, topicARN+":email", "email", map[string]string{})

	if _, err := dmpfsqs.NewSNSPublisher(context.Background(), snsOnlyConfig(), api); !errors.Is(err, dmpfsqs.ErrNoSubscriptions) {
		t.Fatalf("NewSNSPublisher() = %v, want ErrNoSubscriptions", err)
	}
	api.Subscribe(topicARN, subARN, "sqs", map[string]string{"RawMessageDelivery": "true"})
	if _, err := dmpfsqs.NewSNSPublisher(context.Background(), snsOnlyConfig(), api); err != nil {
		t.Fatalf("NewSNSPublisher() = %v, want nil with one raw sqs subscription", err)
	}
}

func TestNewSNSPublisherRefusesAFilterOnAFIFOTopic(t *testing.T) {
	fifoTopic := "arn:aws:sns:us-east-1:000000000000:orders.fifo"
	ch := snsChannel()
	ch.Address = fifoTopic
	ch.Ordering = channel.Ordering{Key: "partitionkey", Unit: channel.Group}
	cfg := validConfig()
	cfg.Catalog = channel.Catalog{ch.Name: ch}

	api := dmpfsqs.NewFakeSNS()
	api.Subscribe(fifoTopic, fifoTopic+":sub-f", "sqs", map[string]string{"RawMessageDelivery": "true", "FilterPolicy": `{"kind":["a"]}`})
	if _, err := dmpfsqs.NewSNSPublisher(context.Background(), cfg, api); !errors.Is(err, dmpfsqs.ErrFilterOnFifoTopic) {
		t.Fatalf("NewSNSPublisher() = %v, want ErrFilterOnFifoTopic (SQS-03c)", err)
	}

	standard := dmpfsqs.NewFakeSNS()
	standard.Subscribe(topicARN, subARN, "sqs", map[string]string{"RawMessageDelivery": "true", "FilterPolicy": `{"kind":["a"]}`})
	if _, err := dmpfsqs.NewSNSPublisher(context.Background(), snsOnlyConfig(), standard); err != nil {
		t.Fatalf("NewSNSPublisher(standard with filter) = %v, want nil: filters are fine on a standard topic", err)
	}
}

func TestSNSPublishSendsTheSameBodyAndAttributes(t *testing.T) {
	api := dmpfsqs.NewFakeSNS()
	api.Subscribe(topicARN, subARN, "sqs", map[string]string{"RawMessageDelivery": "true"})
	pub, err := dmpfsqs.NewSNSPublisher(context.Background(), validConfig(), api)
	if err != nil {
		t.Fatalf("NewSNSPublisher() = %v", err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders-fanout", raw); err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	calls := api.Calls("Publish")
	if len(calls) != 1 {
		t.Fatalf("Publish calls = %d", len(calls))
	}
	in := calls[0].Input.(*sns.PublishInput)
	if *in.TopicArn != topicARN || *in.Message != dmpfsqs.EncodeBody(raw) {
		t.Fatalf("PublishInput = %+v, want the topic and the once-encoded body", in)
	}
	if in.MessageGroupId != nil {
		t.Fatal("a standard topic got FIFO fields")
	}
	if _, ok := in.MessageAttributes[dmpfsqs.AttrPublishedAt]; !ok {
		t.Fatal("the operational attribute is missing")
	}
	if err := pub.Publish(context.Background(), "reservations", raw); !errors.Is(err, dmpfsqs.ErrNotSQSChannel) {
		t.Fatalf("sqs channel through sns = %v, want ErrNotSQSChannel", err)
	}
}
