package main

import (
	"context"
	"os"
	"reflect"
	"time"
	"unsafe"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var endpoint = "localhost:4317"

func main() {
	envEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if envEndpoint != "" {
		endpoint = envEndpoint
	}

	ctx := context.Background()

	otlpmetricExporter, _ := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithEndpoint(endpoint))
	defer otlpmetricExporter.Shutdown(ctx)

	otlplogExporter, _ := otlploggrpc.New(ctx, otlploggrpc.WithInsecure(), otlploggrpc.WithEndpoint(endpoint))
	defer otlplogExporter.Shutdown(ctx)

	otlptraceExporter, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(endpoint))
	defer otlptraceExporter.Shutdown(ctx)

	resource, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("sample-generator"),
			attribute.String("resource-attribute-1", "resource-attribute-value-1"),
		),
	)

	metrics := newMetrics(resource)
	records := newRecords(resource)

	_ = otlpmetricExporter.Export(ctx, metrics)
	_ = otlplogExporter.Export(ctx, records)
	newSpans(resource, otlptraceExporter)
}

func newMetrics(resource *resource.Resource) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{
		Resource: resource,
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: instrumentation.Scope{},
				Metrics: []metricdata.Metrics{
					{
						Name: "custom.metric",
						Data: metricdata.Gauge[float64]{
							DataPoints: []metricdata.DataPoint[float64]{
								{
									Attributes: attribute.NewSet(
										attribute.String("attribute-a", "attribute-a-value-1"),
										attribute.String("job", "custom"),             // For prometheus
										attribute.String("instance", "test-instance"), // For prometheus
									),
									Value: 1.0,
									Time:  time.Now(),
								},
							},
						},
					},
				},
			},
		},
	}
}

func newRecords(resource *resource.Resource) []sdklog.Record {
	record := sdklog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(log.StringValue("log message"))
	record.SetAttributes(log.String("attribute-b", "attribute-b-value-1"))

	setPrivate(&record, "resource", resource)
	return []sdklog.Record{
		record,
	}
}

func newSpans(resource *resource.Resource, traceExporter sdktrace.SpanExporter) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(resource),
	)

	// Manually create a custom TraceID and SpanID for the root span
	traceID := trace.TraceID([16]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
	spanID := trace.SpanID([8]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77})

	ctx := trace.ContextWithRemoteSpanContext(
		context.Background(),
		trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		}),
	)

	// Create and start the root span with the custom TraceID and SpanID
	tracer := tp.Tracer("my-tracer")
	_, rootSpan := tracer.Start(ctx, "root-span", trace.WithAttributes(attribute.String("attribute-c", "attribute-c-value-1")))
	createChildSpan(ctx, tp)

	rootSpan.End()
	tp.ForceFlush(context.Background())
}

func createChildSpan(ctx context.Context, tp *sdktrace.TracerProvider) {
	tracer := tp.Tracer("my-tracer")
	_, childSpan := tracer.Start(ctx, "child-span")
	defer childSpan.End()
}

func setPrivate(record *sdklog.Record, field string, value any) {
	v := reflect.ValueOf(record).Elem()
	setUnexportedField(v.FieldByName(field), value)
}

func setUnexportedField(field reflect.Value, value any) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
		Elem().
		Set(reflect.ValueOf(value))
}
