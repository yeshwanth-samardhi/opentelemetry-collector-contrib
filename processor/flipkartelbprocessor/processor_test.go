package flipkartelbprocessor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestProcessorLogic(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify we received the IP
		if r.URL.Path == "/vips/10.83.36.120" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Group":"default","Kind":"compute#Vip","Users":["/apps/lockin-coinmanager-prod/regions/in-chennai-2/"]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 2. Config
	cfg := &Config{
		Endpoints:     []string{ts.URL + "/vips/{peer_name}"},
		ClientTimeout: "1s",
	}

	// 3. Create Processor
	next := new(consumertest.TracesSink)
	proc, err := newProcessor(zap.NewNop(), cfg, next)
	require.NoError(t, err)

	// 4. Create Trace Data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("net.peer.name", "10.83.36.120")

	// 5. Consume
	err = proc.ConsumeTraces(context.Background(), traces)
	require.NoError(t, err)

	// 6. Verify Result
	processedTraces := next.AllTraces()
	require.Len(t, processedTraces, 1)

	attrs := processedTraces[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	val, ok := attrs.Get("net_peer_app_id")
	require.True(t, ok, "Attribute net_peer_app_id missing")
	assert.Equal(t, "lockin-coinmanager-prod", val.Str())
}
