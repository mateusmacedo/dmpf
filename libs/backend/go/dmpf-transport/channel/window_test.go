package channel_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

func TestKafkaWindow(t *testing.T) {
	t.Run("time retention bounds a delete policy", func(t *testing.T) {
		w := kafkaWindow()
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		if w.UpperBound != 7*24*time.Hour {
			t.Fatalf("UpperBound = %v, want 168h", w.UpperBound)
		}
		if w.Transport() != channel.Kafka {
			t.Fatalf("Transport() = %q, want kafka", w.Transport())
		}
	})

	t.Run("a smaller size horizon decides", func(t *testing.T) {
		w := channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: 7 * 24 * time.Hour,
			RetentionBySize: 1 << 30,
			SizeHorizon:     36 * time.Hour,
			CleanupPolicy:   "delete",
			InitialOffset:   "earliest",
		})
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		if w.UpperBound != 36*time.Hour {
			t.Fatalf("UpperBound = %v, want 36h", w.UpperBound)
		}
	})

	t.Run("compact without delete has no closed bound", func(t *testing.T) {
		w := channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: 7 * 24 * time.Hour,
			RetentionBySize: -1,
			CleanupPolicy:   "compact",
			InitialOffset:   "earliest",
		})
		if err := w.Validate(); !errors.Is(err, channel.ErrInvalidWindow) {
			t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
		}
	})

	t.Run("compact,delete keeps the time bound", func(t *testing.T) {
		w := channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: time.Hour,
			RetentionBySize: -1,
			CleanupPolicy:   "compact,delete",
			InitialOffset:   "latest",
		})
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	for _, missing := range []string{"retentionByTime", "retentionBySize", "cleanupPolicy", "initialOffset"} {
		t.Run("without "+missing, func(t *testing.T) {
			r := channel.KafkaRetention{
				RetentionByTime: time.Hour,
				RetentionBySize: -1,
				CleanupPolicy:   "delete",
				InitialOffset:   "earliest",
			}
			switch missing {
			case "retentionByTime":
				r.RetentionByTime = 0
			case "retentionBySize":
				r.RetentionBySize = 0
			case "cleanupPolicy":
				r.CleanupPolicy = ""
			case "initialOffset":
				r.InitialOffset = ""
			}

			err := channel.KafkaWindow(r).Validate()
			if !errors.Is(err, channel.ErrInvalidWindow) {
				t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Fatalf("Validate() = %q, want it to name %q", err, missing)
			}
		})
	}
}

func TestSQSWindow(t *testing.T) {
	t.Run("receipts times visibility when below retention", func(t *testing.T) {
		w := channel.SQSWindow(5, 30*time.Second, 4*24*time.Hour)
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		if w.UpperBound != 150*time.Second {
			t.Fatalf("UpperBound = %v, want 150s", w.UpperBound)
		}
	})

	t.Run("retention when smaller", func(t *testing.T) {
		w := channel.SQSWindow(5, 30*time.Second, 60*time.Second)
		if w.UpperBound != 60*time.Second {
			t.Fatalf("UpperBound = %v, want 60s", w.UpperBound)
		}
	})

	t.Run("visibility is capped at twelve hours per attempt", func(t *testing.T) {
		w := channel.SQSWindow(2, 20*time.Hour, 14*24*time.Hour)
		if w.UpperBound != 2*channel.MaxVisibility {
			t.Fatalf("UpperBound = %v, want 24h", w.UpperBound)
		}
	})

	t.Run("a missing parameter is refused", func(t *testing.T) {
		for name, w := range map[string]channel.RedeliveryWindow{
			"maxReceiveCount": channel.SQSWindow(0, 30*time.Second, time.Hour),
			"visibilityBase":  channel.SQSWindow(5, 0, time.Hour),
			"retention":       channel.SQSWindow(5, 30*time.Second, 0),
		} {
			err := w.Validate()
			if !errors.Is(err, channel.ErrInvalidWindow) {
				t.Fatalf("%s: Validate() = %v, want ErrInvalidWindow", name, err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("%s: Validate() = %q, want it to name the parameter", name, err)
			}
		}
	})
}

func TestSNSSQSWindow(t *testing.T) {
	sqs := channel.SQSWindow(5, 30*time.Second, 4*24*time.Hour)

	t.Run("sums the two windows", func(t *testing.T) {
		w := channel.SNSSQSWindow(time.Hour, sqs)
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		if want := time.Hour + sqs.UpperBound; w.UpperBound != want {
			t.Fatalf("UpperBound = %v, want %v", w.UpperBound, want)
		}
		if w.Transport() != channel.SNSSQS {
			t.Fatalf("Transport() = %q, want sns-sqs", w.Transport())
		}
	})

	t.Run("carries the queue parameters", func(t *testing.T) {
		w := channel.SNSSQSWindow(time.Hour, sqs)
		for _, name := range []string{"sqs.maxReceiveCount", "sqs.visibilityBase", "sqs.retention", "sqs.upperBound", "subscriptionDeliveryWindow"} {
			if w.Params[name] == "" {
				t.Errorf("Params[%q] is empty", name)
			}
		}
	})

	t.Run("without a delivery window", func(t *testing.T) {
		if err := channel.SNSSQSWindow(0, sqs).Validate(); !errors.Is(err, channel.ErrInvalidWindow) {
			t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
		}
	})

	t.Run("with an invalid queue window", func(t *testing.T) {
		w := channel.SNSSQSWindow(time.Hour, channel.SQSWindow(0, 0, 0))
		if err := w.Validate(); !errors.Is(err, channel.ErrInvalidWindow) {
			t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
		}
	})

	t.Run("with a window of another transport", func(t *testing.T) {
		w := channel.SNSSQSWindow(time.Hour, kafkaWindow())
		if err := w.Validate(); !errors.Is(err, channel.ErrInvalidWindow) {
			t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
		}
	})
}

func TestRedeliveryWindowValidate(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var w channel.RedeliveryWindow
		if !w.IsZero() {
			t.Fatal("IsZero() = false, want true")
		}
		if err := w.Validate(); !errors.Is(err, channel.ErrInvalidWindow) {
			t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
		}
	})

	t.Run("unknown formula", func(t *testing.T) {
		w := channel.RedeliveryWindow{Formula: "depends on configuration", UpperBound: time.Hour}
		if err := w.Validate(); !errors.Is(err, channel.ErrInvalidWindow) {
			t.Fatalf("Validate() = %v, want ErrInvalidWindow", err)
		}
	})
}
