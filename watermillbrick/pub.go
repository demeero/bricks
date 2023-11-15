package watermillbrick

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
	wotelfloss "github.com/dentech-floss/watermill-opentelemetry-go-extra/pkg/opentelemetry"
	wotel "github.com/voi-oss/watermill-opentelemetry/pkg/opentelemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

type OTELPubConfig struct {
	Name                string
	Metrics             bool
	NewRootSpanWithLink bool
}

type OTELPublisher struct {
	pub               message.Publisher
	evtPublishCounter metric.Int64Counter
	cfg               OTELPubConfig
}

func NewOTELPublisher(cfg OTELPubConfig, pub message.Publisher) (*OTELPublisher, error) {
	pub = wotelfloss.NewTracePropagatingPublisherDecorator(pub)
	pub = wotel.NewNamedPublisherDecorator(fmt.Sprintf("%s.publish", cfg.Name), pub)
	var counter metric.Int64Counter
	if cfg.Metrics {
		m, err := otel.GetMeterProvider().Meter("bricks/watermillbrick/pub").
			Int64Counter("event_publish_count", metric.WithDescription("The number of events published"))
		if err != nil {
			return nil, fmt.Errorf("failed to create event_publish_count metric: %w", err)
		}
		counter = m
	}

	return &OTELPublisher{pub: pub, evtPublishCounter: counter, cfg: cfg}, nil
}

func (p *OTELPublisher) Publish(topic string, messages ...*message.Message) error {
	if len(messages) == 0 {
		return nil
	}
	ctx := messages[0].Context()
	if p.cfg.NewRootSpanWithLink {
		spanCtx, span := otel.GetTracerProvider().Tracer("bricks/publisher").
			Start(ctx, "publish",
				trace.WithNewRoot(),
				trace.WithLinks(trace.Link{SpanContext: trace.SpanContextFromContext(ctx)}))
		ctx = spanCtx
		defer span.End()
		for _, msg := range messages {
			msg.SetContext(ctx)
		}
	}

	err := p.pub.Publish(topic, messages...)
	p.recordMetrics(ctx, topic, err)
	return err
}

func (p *OTELPublisher) Close() error {
	return p.pub.Close()
}

func (p *OTELPublisher) recordMetrics(ctx context.Context, topic string, err error) {
	if !p.cfg.Metrics {
		return
	}
	var resultAttr attribute.KeyValue
	if err != nil {
		resultAttr = semconv.OTelStatusCodeError
	} else {
		resultAttr = semconv.OTelStatusCodeOk
	}
	p.evtPublishCounter.Add(ctx, 1, metric.WithAttributes(semconv.MessagingDestinationName(topic), resultAttr))
}
