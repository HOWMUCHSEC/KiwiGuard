// Package httpprovider adapts external HTTP services to verdict providers.
package httpprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	domainverdict "github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

// HTTPProviderOptions defines endpoint, credentials, and transport settings for the HTTP verdict provider.
type HTTPProviderOptions struct {
	Name       string
	Endpoint   string
	APIKey     string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type httpProvider struct {
	name       string
	endpoint   string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

type httpProviderRequest struct {
	RequestID     string                  `json:"request_id"`
	CorrelationID string                  `json:"correlation_id"`
	RouteKey      string                  `json:"route_key"`
	ProviderKey   string                  `json:"provider_key"`
	Model         string                  `json:"model"`
	Direction     domainverdict.Direction `json:"direction"`
	Text          string                  `json:"text"`
	Metadata      map[string]string       `json:"metadata,omitempty"`
}

type httpProviderMatchedSpan struct {
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Category string `json:"category"`
	TextHash string `json:"text_hash"`
}

type httpProviderResult struct {
	RiskLevel       domainverdict.RiskLevel   `json:"risk_level"`
	Categories      []string                  `json:"categories"`
	Confidence      float64                   `json:"confidence"`
	SuggestedAction domainverdict.Action      `json:"suggested_action"`
	MatchedSpans    []httpProviderMatchedSpan `json:"matched_spans,omitempty"`
	Rationale       string                    `json:"rationale,omitempty"`
	ProviderName    string                    `json:"provider_name"`
	FallbackUsed    bool                      `json:"fallback_used"`
	Error           string                    `json:"error,omitempty"`
}

// NewHTTPProvider returns a provider backed by a structured HTTP endpoint.
func NewHTTPProvider(opts HTTPProviderOptions) domainverdict.Provider {
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return httpProvider{
		name:       opts.Name,
		endpoint:   opts.Endpoint,
		apiKey:     opts.APIKey,
		httpClient: client,
		timeout:    opts.Timeout,
	}
}

func (p httpProvider) Evaluate(ctx context.Context, request domainverdict.Request) (domainverdict.Result, error) {
	if p.endpoint == "" {
		return domainverdict.Result{}, errors.New("verdict http endpoint is required")
	}

	start := time.Now()
	if p.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	body, err := json.Marshal(httpProviderRequestFromDomain(request))
	if err != nil {
		return domainverdict.Result{}, fmt.Errorf("encode verdict request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return domainverdict.Result{}, fmt.Errorf("create verdict request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	if p.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	response, err := p.httpClient.Do(httpRequest)
	if err != nil {
		return domainverdict.Result{}, fmt.Errorf("send verdict request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return domainverdict.Result{}, fmt.Errorf("verdict http status %d", response.StatusCode)
	}

	var wireResult httpProviderResult
	if err := json.NewDecoder(response.Body).Decode(&wireResult); err != nil {
		return domainverdict.Result{}, fmt.Errorf("decode verdict response: %w", err)
	}
	result := wireResult.domainResult()
	result.Latency = time.Since(start)
	if result.Latency <= 0 {
		result.Latency = time.Nanosecond
	}
	if result.ProviderName == "" {
		result.ProviderName = p.name
	}

	return result, nil
}

func httpProviderRequestFromDomain(request domainverdict.Request) httpProviderRequest {
	return httpProviderRequest{
		RequestID:     request.RequestID,
		CorrelationID: request.CorrelationID,
		RouteKey:      request.RouteKey,
		ProviderKey:   request.ProviderKey,
		Model:         request.Model,
		Direction:     request.Direction,
		Text:          request.Text,
		Metadata:      request.Metadata,
	}
}

func (r httpProviderResult) domainResult() domainverdict.Result {
	var matchedSpans []domainverdict.MatchedSpan
	if len(r.MatchedSpans) > 0 {
		matchedSpans = make([]domainverdict.MatchedSpan, 0, len(r.MatchedSpans))
		for _, span := range r.MatchedSpans {
			matchedSpans = append(matchedSpans, domainverdict.MatchedSpan{
				Start:    span.Start,
				End:      span.End,
				Category: span.Category,
				TextHash: span.TextHash,
			})
		}
	}

	return domainverdict.Result{
		RiskLevel:       r.RiskLevel,
		Categories:      r.Categories,
		Confidence:      r.Confidence,
		SuggestedAction: r.SuggestedAction,
		MatchedSpans:    matchedSpans,
		Rationale:       r.Rationale,
		ProviderName:    r.ProviderName,
		FallbackUsed:    r.FallbackUsed,
		Error:           r.Error,
	}
}
