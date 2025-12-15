package flipkartelbprocessor

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Response matches the JSON structure you provided
type VipResponse struct {
	Users []string `json:"Users"`
}

type flipkartElbProcessor struct {
	logger     *zap.Logger
	config     *Config
	next       consumer.Traces
	httpClient *http.Client

	// simple cache to avoid spamming the API for the same IP
	// Key: net.peer.name, Value: AppID
	// In production, use an LRU cache to avoid memory leaks!
	cache        map[string]string
	cacheMutex   sync.RWMutex
	requestGroup singleflight.Group // Add this
}

func newProcessor(logger *zap.Logger, cfg *Config, next consumer.Traces) (processor.Traces, error) {
	// Parse timeout
	timeout, err := time.ParseDuration(cfg.ClientTimeout)
	if err != nil {
		timeout = 500 * time.Millisecond // default
	}

	return &flipkartElbProcessor{
		logger: logger,
		config: cfg,
		next:   next,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cache: make(map[string]string),
	}, nil
}

func (p *flipkartElbProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *flipkartElbProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (p *flipkartElbProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (p *flipkartElbProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				p.processSpan(span)
			}
		}
	}
	return p.next.ConsumeTraces(ctx, td)
}

func (p *flipkartElbProcessor) processSpan(span ptrace.Span) {
	peerNameVal, ok := span.Attributes().Get("net.peer.name")
	if !ok {
		return
	}
	peerName := peerNameVal.Str()

	// 1. FAST PATH: Check Cache
	p.cacheMutex.RLock()
	cachedAppID, found := p.cache[peerName]
	p.cacheMutex.RUnlock()

	if found {
		if cachedAppID != "" {
			span.Attributes().PutStr("net_peer_app_id", cachedAppID)
		}
		return
	}

	// 2. SINGLEFLIGHT: Coalesce multiple requests for the same IP
	// "Do" ensures only one function runs for the given key (peerName)
	// Other threads wait here for the result to be ready.
	result, err, _ := p.requestGroup.Do(peerName, func() (interface{}, error) {
		// This function runs ONLY ONCE per concurrent key
		return p.resolveAppID(peerName), nil
	})

	appID := result.(string)

	// 3. Update Cache safely
	p.cacheMutex.Lock()
	p.cache[peerName] = appID
	p.cacheMutex.Unlock()

	if appID != "" {
		span.Attributes().PutStr("net_peer_app_id", appID)
	}
}

func (p *flipkartElbProcessor) resolveAppID(peerName string) string {
	for _, endpointTmpl := range p.config.Endpoints {
		// Replace placeholder with actual peer name
		url := strings.ReplaceAll(endpointTmpl, "{peer_name}", peerName)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			p.logger.Debug("Failed to create request", zap.Error(err))
			continue
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			p.logger.Debug("Request failed", zap.String("url", url), zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue // Try next endpoint
		}

		var vipResp VipResponse
		if err := json.NewDecoder(resp.Body).Decode(&vipResp); err != nil {
			p.logger.Error("Failed to decode JSON", zap.Error(err))
			continue
		}

		// Logic: Get data after /apps/
		// Example User: "/apps/lockin-coinmanager-prod/regions/..."
		if len(vipResp.Users) > 0 {
			userPath := vipResp.Users[0]
			parts := strings.Split(userPath, "/")

			// parts[0] = "" (leading slash)
			// parts[1] = "apps"
			// parts[2] = "lockin-coinmanager-prod" <-- This is what we want
			if len(parts) > 2 && parts[1] == "apps" {
				return parts[2]
			}
		}
	}
	return ""
}
