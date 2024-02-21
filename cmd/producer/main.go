package main

import (
	"context"
	"fmt"
	uuid2 "github.com/google/uuid"
	"github.com/uber/jaeger-client-go"
	thriftgen "github.com/uber/jaeger-client-go/thrift-gen/jaeger"
	"github.com/uber/jaeger-client-go/utils"
	"math/rand"
	"time"
)

func main() {

	//addr, err := net.ResolveUDPAddr("udp", "localhost:6831")
	//if err != nil {
	//	panic(err)
	//}
	//conn, err := net.DialUDP("udp", nil, addr)
	//if err != nil {
	//	panic(err)
	//}
	//defer conn.Close()

	client, err := utils.NewAgentClientUDP("localhost:6831", utils.UDPPacketMaxLength)
	if err != nil {
		return
	}
	defer client.Close()

	batch := thriftgen.NewBatch()
	batch.Process = thriftgen.NewProcess()

	ver := jaeger.JaegerClientVersion
	host := "PUT_HOST_HERE"
	ip := "PUT_IP_HERE"
	uuid := uuid2.NewString()

	i := int64(1)

	batch.Process.ServiceName = "sso"
	batch.Process.Tags = []*thriftgen.Tag{
		&thriftgen.Tag{
			Key:   jaeger.JaegerClientVersionTagKey,
			VType: thriftgen.TagType_STRING,
			VStr:  &ver,
		},
		&thriftgen.Tag{
			Key:   jaeger.TracerHostnameTagKey,
			VType: thriftgen.TagType_STRING,
			VStr:  &host,
		}, &thriftgen.Tag{
			Key:   jaeger.TracerIPTagKey,
			VType: thriftgen.TagType_STRING,
			VStr:  &ip,
		}, &thriftgen.Tag{
			Key:   jaeger.TracerUUIDTagKey,
			VType: thriftgen.TagType_STRING,
			VStr:  &uuid,
		},
	}
	batch.SeqNo = &i
	batch.Spans = []*thriftgen.Span{
		&thriftgen.Span{
			TraceIdLow:    rand.Int63(),
			TraceIdHigh:   0,
			SpanId:        rand.Int63(),
			ParentSpanId:  0,
			OperationName: "HTTP POST URL: /login",
			Flags:         1,
			References:    make([]*thriftgen.SpanRef, 0),
			StartTime:     utils.TimeToMicrosecondsSinceEpochInt64(time.Now()),
			Duration:      1 * time.Second.Nanoseconds() / int64(time.Microsecond),
			Tags: []*thriftgen.Tag{
				&thriftgen.Tag{
					Key:   "k1",
					VType: thriftgen.TagType_STRING,
					VStr:  &ver,
				},
			},
			Logs: make([]*thriftgen.Log, 0),
		},
	}
	batch.Stats = &thriftgen.ClientStats{
		FullQueueDroppedSpans: 0,
		TooLargeDroppedSpans:  0,
		FailedToEmitSpans:     0,
	}

	err = client.EmitBatch(context.Background(), batch)
	if err != nil {
		return
	}

	fmt.Println("Sent span to Jaeger agent")
}
