package main

import (
	"net/http"
	"log"
	"github.com/openzipkin/zipkin-go"
	httpreporter "github.com/openzipkin/zipkin-go/reporter/http"
	zipkinhttp "github.com/openzipkin/zipkin-go/middleware/http"
	httpclient "github.com/openzipkin/zipkin-go/middleware/http"
	"time"
)

var tracer *zipkin.Tracer

func main() {
	reporter := httpreporter.NewReporter("http://host.docker.internal:9411/api/v2/spans")
	defer reporter.Close()

	endpoint, err := zipkin.NewEndpoint("srv4", "host.docker.internal:65515")
	if err != nil {
		log.Fatalf("unable to create local endpoint: %+v\n", err)
	}

	tracer, err = zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(endpoint))
	if err != nil {
		log.Fatalf("unable to create tracer: %+v\n", err)
	}

	// 使用middleware自动处理server端投递span
	serverMiddleware := zipkinhttp.NewServerMiddleware(
		tracer,
		zipkinhttp.TagResponseSize(true),
		zipkinhttp.SpanName("serve srv4"),
	)

	f := &Foo{}
	http.Handle("/test", serverMiddleware(f))
	http.ListenAndServe(":65515", nil)
}

type Foo struct {}

func (f *Foo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	<- time.After(2 * time.Second)

	clientTags := map[string]string{
		"client": "testClient",
	}

	transportTags := map[string]string{
		"conf.timeout": "default",
	}

	client, err := httpclient.NewClient(
		tracer,
		httpclient.WithClient(&http.Client{}),
		httpclient.ClientTrace(true),
		httpclient.ClientTags(clientTags),
		httpclient.TransportOptions(httpclient.TransportTags(transportTags)),
	)
	if err != nil {
		log.Fatalf("unable to create http client: %+v", err)
	}

	// 向srv5发起请求
	req, _ := http.NewRequest("GET", "http://host.docker.internal:65516/test", nil)
	req = req.WithContext(r.Context())
	res, err := client.DoWithAppSpan(req, "get srv5")
	if err != nil {
		log.Fatalf("unable to execute client request: %+v", err)
	}
	res.Body.Close()

	w.Write([]byte("response from srv4"))
}