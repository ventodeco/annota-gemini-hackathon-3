package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gemini-hackathon/app/internal/knowledge"
)

const annotationFixture = `{"translation":"hello","contextual_explanation":"A greeting","usage_example":"こんにちは。","when_to_use":"Daytime","word_breakdown":{"こんにちは":"hello"},"alternative_meanings":"None","pronunciation":{"kana":"こんにちは"}}`

func testGateway(t *testing.T, server *httptest.Server, kind, task string, opts RoutedClientOptions) *taskGateway {
	t.Helper()
	cfg := routingFixture(t)
	p := cfg.Profiles["active"]
	p.BaseURL = server.URL + "/v1"
	p.Kind = kind
	cfg.Profiles["active"] = p
	return newTaskGateway(cfg, cfg.Tasks.named()[task], task, opts)
}

func TestChatTasksRouteAndNormalize(t *testing.T) {
	for _, kind := range []string{"openrouter", "modelverse"} {
		t.Run(kind, func(t *testing.T) {
			var events []CallMetadata
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Errorf("wrong endpoint/auth")
				}
				var body struct {
					Model          string          `json:"model"`
					Messages       json.RawMessage `json:"messages"`
					Provider       json.RawMessage `json:"provider"`
					ResponseFormat json.RawMessage `json:"response_format"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Model != "test-vision" || len(body.ResponseFormat) == 0 {
					t.Error("missing model/structured output")
				}
				if kind == "modelverse" && len(body.Provider) > 0 {
					t.Error("OpenRouter fields leaked to ModelVerse")
				}
				text := `{"raw_text":"日本語","language":"ja"}`
				if !strings.Contains(string(body.Messages), "data:image/png;base64,") {
					if !strings.Contains(string(body.Messages), "辞書") || !strings.Contains(string(body.Messages), "こんにちは") {
						t.Error("lost knowledge/selection")
					}
					text = annotationFixture
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "gen-fixture", "model": "test-vision", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": text}}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20, "cost": 0.001}})
			}))
			defer server.Close()
			opts := RoutedClientOptions{Observe: func(_ context.Context, m CallMetadata) { events = append(events, m) }}
			tasks := chatTasks{ocr: testGateway(t, server, kind, "ocr", opts), annotation: testGateway(t, server, kind, "annotation", opts)}
			got, err := tasks.OCR(context.Background(), []byte("page"), "image/png")
			if err != nil {
				t.Fatal(err)
			}
			if got.RawText != "日本語" || got.Language != "ja" || !strings.Contains(got.StructuredJSON, "raw_text") {
				t.Fatalf("bad OCR: %+v", got)
			}
			ann, err := tasks.AnnotateWithKnowledge(context.Background(), "page", "こんにちは", []knowledge.Entry{{Kosakata: "辞書"}})
			if err != nil {
				t.Fatal(err)
			}
			if ann.Meaning != "A greeting" || !strings.Contains(ann.WordBreakdown, "hello") {
				t.Fatalf("bad annotation: %+v", ann)
			}
			if len(events) != 2 || events[0].ReportedModel != "test-vision" || events[0].InputTokens == nil || *events[0].InputTokens != 10 {
				t.Fatalf("lost metadata: %+v", events)
			}
		})
	}
}

func TestChatTasksRejectBadResponses(t *testing.T) {
	cases := []struct {
		name, body string
		annotation bool
	}{
		{"empty choices", `{"choices":[]}`, false},
		{"business error", `{"error":{"message":"private-page fixture-key"}}`, false},
		{"truncated", `{"choices":[{"finish_reason":"length","message":{"content":"private-page"}}]}`, false},
		{"refusal", `{"choices":[{"finish_reason":"stop","message":{"refusal":"private-page","content":"private-page"}}]}`, false},
		{"wrong model", `{"model":"wrong-model","choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`, false},
		{"malformed OCR", `{"choices":[{"finish_reason":"stop","message":{"content":"{bad-json"}}]}`, false},
		{"null annotation", `{"choices":[{"finish_reason":"stop","message":{"content":"null"}}]}`, true},
		{"incomplete annotation", `{"choices":[{"finish_reason":"stop","message":{"content":"{\"translation\":\"hello\"}"}}]}`, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tt.body)) }))
			defer server.Close()
			var events []CallMetadata
			opts := RoutedClientOptions{Observe: func(_ context.Context, m CallMetadata) { events = append(events, m) }}
			tasks := chatTasks{ocr: testGateway(t, server, "openrouter", "ocr", opts), annotation: testGateway(t, server, "openrouter", "annotation", opts)}
			var err error
			if tt.annotation {
				_, err = tasks.Annotate(context.Background(), "private-page", "hello")
			} else {
				_, err = tasks.OCR(context.Background(), []byte("private-page"), "image/png")
			}
			var pe *ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("wanted typed failure, got %v", err)
			}
			eventJSON, _ := json.Marshal(events)
			if strings.Contains(err.Error()+string(eventJSON), "private-page") || strings.Contains(err.Error()+string(eventJSON), "fixture-key") {
				t.Fatal("leaked private content")
			}
			if len(events) != 1 || events[0].ErrorKind == "" {
				t.Fatal("failed output not observed")
			}
		})
	}
}

func TestChatTasksMarkdownIsExplicit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": "# 日本語\n本文"}}}})
	}))
	defer server.Close()
	g := testGateway(t, server, "modelverse", "ocr", RoutedClientOptions{})
	g.model.OCROutput = "markdown"
	tasks := chatTasks{ocr: g}
	got, err := tasks.OCR(context.Background(), []byte("page"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if got.RawText != "# 日本語\n本文" {
		t.Fatal("lost Markdown formatting")
	}
}

func TestGatewayHTTPStatusRetriesAndCancellation(t *testing.T) {
	for _, status := range []int{401, 402, 403, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("private-page fixture-key"))
			}))
			defer server.Close()
			g := testGateway(t, server, "openrouter", "ocr", RoutedClientOptions{})
			g.profile.MaxAttempts = 2
			_, err := (&chatTasks{ocr: g}).OCR(context.Background(), []byte("page"), "image/png")
			var pe *ProviderError
			if !errors.As(err, &pe) {
				t.Fatal(err)
			}
			want := int32(1)
			if status == 429 || status == 503 {
				want = 2
			}
			if calls.Load() != want {
				t.Fatalf("requests=%d want=%d", calls.Load(), want)
			}
			kinds := map[int]ErrorKind{401: ErrorKindAuth, 402: ErrorKindQuota, 403: ErrorKindAuth, 429: ErrorKindRateLimit, 503: ErrorKindUpstream}
			if pe.Kind != kinds[status] || strings.Contains(err.Error(), "private-page") {
				t.Fatalf("unsafe/wrong error: %v", err)
			}
		})
	}
	t.Run("deadline", func(t *testing.T) {
		var calls atomic.Int32
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); <-release }))
		defer server.Close()
		defer close(release)
		g := testGateway(t, server, "openrouter", "ocr", RoutedClientOptions{})
		g.profile.MaxAttempts = 2
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err := (&chatTasks{ocr: g}).OCR(ctx, []byte("page"), "image/png")
		if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
			t.Fatalf("deadline/request count: %v / %d", err, calls.Load())
		}
	})
}

func TestGatewayHTTPRejectsRedirectsAndOversizedBodies(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinationCalls.Add(1) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	g := testGateway(t, server, "openrouter", "ocr", RoutedClientOptions{HTTPClient: server.Client()})
	if _, err := (&chatTasks{ocr: g}).OCR(context.Background(), []byte("page"), "image/png"); err == nil || destinationCalls.Load() != 0 {
		t.Fatal("redirect followed")
	}
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 8<<20+1))) }))
	defer big.Close()
	if _, err := (&chatTasks{ocr: testGateway(t, big, "openrouter", "ocr", RoutedClientOptions{})}).OCR(context.Background(), []byte("page"), "image/png"); err == nil {
		t.Fatal("accepted oversized response")
	}
}

func TestChatTasksClassifiesEmbeddedQuotaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":402,"message":"private-page fixture-key"}}`))
	}))
	defer server.Close()
	_, err := (&chatTasks{ocr: testGateway(t, server, "openrouter", "ocr", RoutedClientOptions{})}).OCR(context.Background(), []byte("page"), "image/png")
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Kind != ErrorKindQuota {
		t.Fatalf("embedded quota error lost: %v", err)
	}
}
