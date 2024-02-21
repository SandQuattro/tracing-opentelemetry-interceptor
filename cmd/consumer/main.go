package main

import (
	"context"
	"encoding/hex"
	"fmt"
	logdoc "github.com/LogDoc-org/logdoc-go-appender/logrus"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go/thrift"
	"github.com/uber/jaeger-client-go/thrift-gen/agent"
	"github.com/uber/jaeger-client-go/thrift-gen/jaeger"
	"github.com/uber/jaeger-client-go/utils"
	"go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"log"
	"net"
	"sync"
	"time"
)

type server struct {
	v1.UnimplementedTraceServiceServer
}

func main() {

	// Создаем подсистему логгирования LogDoc
	_, err := logdoc.Init("tcp", "127.0.0.1:5656", "logdoc-jaeger-interceptor")
	if err != nil {
		log.Println(err)
	}

	logger := logdoc.GetLogger()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go processThriftCompactProtocol(logger)
	go processOLTPgRPCProtocol()
	wg.Wait()
}

func processThriftCompactProtocol(logger *logrus.Logger) {
	ctx := context.Background()

	addr, err := net.ResolveUDPAddr("udp", ":6831")
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	log.Println(">> LISTENING TRACES ON 6831...")

	for {
		buf := make([]byte, utils.UDPPacketMaxLength)
		_, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("received: %x", buf)

		go func() {
			transport := thrift.NewTMemoryBufferLen(utils.UDPPacketMaxLength)
			_, err = transport.Write(buf)
			if err != nil {
				log.Fatal(err)
			}

			protocol := thrift.NewTCompactProtocol(transport)
			begin, t, i, err := protocol.ReadMessageBegin(ctx)
			if err != nil {
				log.Fatal(err)
			}

			log.Println(begin, t, i)

			args := agent.AgentEmitBatchArgs{Batch: jaeger.NewBatch()}

			if err = args.Read(ctx, protocol); err != nil {
				log.Println(err)
			}

			for _, span := range args.Batch.Spans {
				fmt.Println("Span:", span.OperationName)
				fmt.Println("  Trace ID:", span.TraceIdHigh, span.TraceIdLow)
				fmt.Println("  Span ID:", span.SpanId)
				fmt.Println("  Start time:", span.StartTime)
				fmt.Println("  Duration:", span.Duration)
				fmt.Println("  Tags:", span.Tags)
				fmt.Println("  Logs:", span.Logs)
				logger.Debug("Incoming jaeger trace, operation:", span.OperationName, "@@operation=", span.OperationName, "@TraceID=", span.TraceIdLow, "@SpanID=", span.SpanId,
					"@StartTime=", span.StartTime, "@Duration=", span.Duration, "@Tags=", span.Tags, "@Logs=", span.Logs,
				)
			}
		}()
	}
}

func (s *server) Export(ctx context.Context, req *v1.ExportTraceServiceRequest) (*v1.ExportTraceServiceResponse, error) {
	logger := logdoc.GetLogger()
	log.Println("Received traces...")
	go func() {
		for _, resourceSpan := range req.GetResourceSpans() {
			log.Printf("ResourceSpans: %d spans", len(resourceSpan.GetScopeSpans()))
			for _, resource := range resourceSpan.GetResource().GetAttributes() {
				log.Println("Resource Attribute Key:", resource.Key, "Value:", resource.Value.GetStringValue())
			}
			for _, scopeSpan := range resourceSpan.GetScopeSpans() {
				fmt.Println("  Tracer:", scopeSpan.Scope.Name)
				fmt.Println("  Version:", scopeSpan.Scope.Version)
				fmt.Println("  Tracer attributes:", scopeSpan.Scope.Attributes)
				for _, span := range scopeSpan.Spans {
					fmt.Println("Span:", span.Name)
					fmt.Println("  Trace ID:", bytesZeroCheck(span.TraceId))
					fmt.Println("  Span ID:", bytesZeroCheck(span.SpanId))
					fmt.Println("  Parent Span ID:", bytesZeroCheck(span.ParentSpanId))
					const layout = "2006-01-02 15:04:05.000 MST"
					startTime := convertNanoToTime(int64(span.StartTimeUnixNano))
					endTime := convertNanoToTime(int64(span.EndTimeUnixNano))
					duration := endTime.Sub(startTime)
					fmt.Println("Formatted start time:", startTime.Format(layout))
					fmt.Println("Formatted end time:", endTime.Format(layout))
					fmt.Println("Formatted duration in seconds:", duration.Seconds())
					fmt.Println("  Attributes:", span.GetAttributes())
					for _, attr := range span.GetAttributes() {
						fmt.Println("  Attribute Key:", attr.Key, "Value:", attr.Value)
					}
					fmt.Println("  Logs:", span.GetEvents())
					for _, event := range span.GetEvents() {
						fmt.Println("  Log ", event.Name)
						for _, attr := range event.GetAttributes() {
							fmt.Println("  event attribute Key:", attr.Key, "Value:", attr.Value)
						}
					}
					fmt.Println("  Links:", span.GetLinks())
					logger.Debug("Incoming jaeger OLTP trace over gRPC, operation:", span.Name,
						"@@operation=", span.Name,
						"@TraceID=", bytesZeroCheck(span.TraceId),
						"@ParentSpanID=", bytesZeroCheck(span.ParentSpanId),
						"@SpanID=", bytesZeroCheck(span.SpanId),
						"@StartTime=", startTime.Format(layout),
						"@EndTime=", endTime.Format(layout),
						"@Duration=", duration.Milliseconds(),
						"@Tags=", span.Attributes,
					)
				}
			}
		}
	}()

	return &v1.ExportTraceServiceResponse{
		PartialSuccess: &v1.ExportTracePartialSuccess{
			RejectedSpans: 0,
			ErrorMessage:  "",
		},
	}, nil
}

// OLTP over gRPC
func processOLTPgRPCProtocol() {
	lis, err := net.Listen("tcp", ":4317")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf(">> LISTENING FOR OTLP ON %s", lis.Addr().String())

	s := grpc.NewServer()
	v1.RegisterTraceServiceServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func convertNanoToTime(nano int64) time.Time {
	return time.Unix(0, nano)
}

func bytesZeroCheck(val []byte) string {
	if len(val) == 0 {
		return "0"
	}
	return hex.EncodeToString(val)
}
