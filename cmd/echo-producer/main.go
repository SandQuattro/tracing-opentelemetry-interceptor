package main

import (
	"context"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"log"
)

func main() {
	InitTracer()

	e := echo.New()

	e.Use(otelecho.Middleware("echo-producer"))
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		ctx := c.Request().Context()
		trace.SpanFromContext(ctx).RecordError(err)

		e.DefaultHTTPErrorHandler(err, c)
	}

	e.GET("/", func(ctx echo.Context) error {
		tracer := otel.Tracer("root tracer")
		_, span := tracer.Start(context.Background(), "Root Handler Span",
			oteltrace.WithAttributes(semconv.ServiceName("echo-producer")),
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		)
		//span.SetAttributes()
		defer span.End()

		name := ctx.QueryParam("name")
		if name == "" {
			name = "world"
		}

		span.SetAttributes(attribute.String("name", name))

		return ctx.String(200, "Hello, World!")
	})

	err := e.Start(":8080")
	if err != nil {
		log.Fatal(err)
	}
}

func InitTracer() {
	exporter, err := jaeger.New(
		jaeger.WithAgentEndpoint(jaeger.WithAgentHost("localhost"), jaeger.WithAgentPort("6831")),
		//jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")),
	)
	if err != nil {
		log.Fatal(err)
	}

	stdoutexporter, err := stdout.New(stdout.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithBatcher(stdoutexporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			"echo-producer", semconv.ServiceNameKey.String("echo-producer"),
		)))

	otel.SetTracerProvider(tp)
}
