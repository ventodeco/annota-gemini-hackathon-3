package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type mockSpeechAIClient struct {
	resp       *ai.SpeechResponse
	err        error
	annotation *ai.AnnotationResponse
	annErr     error
}

func (m *mockSpeechAIClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*ai.OCRResponse, error) {
	return nil, nil
}

func (m *mockSpeechAIClient) Annotate(ctx context.Context, ocrText string, selectedText string) (*ai.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockSpeechAIClient) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*ai.AnnotationResponse, error) {
	return m.annotation, m.annErr
}

func (m *mockSpeechAIClient) SynthesizeSpeech(ctx context.Context, highlightedText string, contextText string) (*ai.SpeechResponse, error) {
	return m.resp, m.err
}

func TestSpeakAPI(t *testing.T) {
	t.Run("returns unauthorized when user is missing", func(t *testing.T) {
		h := handlers.NewAIHandlers(testutil.NewMockDB(), &mockSpeechAIClient{}, knowledge.NewEmptyService())
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", strings.NewReader(`{"highlightedText":"テスト"}`))
		rec := httptest.NewRecorder()

		h.SpeakAPI(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("returns bad request when highlighted text is empty", func(t *testing.T) {
		h := handlers.NewAIHandlers(testutil.NewMockDB(), &mockSpeechAIClient{}, knowledge.NewEmptyService())
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", strings.NewReader(`{"highlightedText":""}`))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()

		h.SpeakAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns binary audio when synthesis succeeds", func(t *testing.T) {
		h := handlers.NewAIHandlers(
			testutil.NewMockDB(),
			&mockSpeechAIClient{
				resp: &ai.SpeechResponse{
					Audio:    []byte{0x52, 0x49, 0x46, 0x46},
					MIMEType: "audio/wav",
				},
			},
			knowledge.NewEmptyService(),
		)
		body := bytes.NewBufferString(`{"highlightedText":"おはよう","contextText":"丁寧に挨拶する場面","tone":"ignored"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", body)
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()

		h.SpeakAPI(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "audio/wav" {
			t.Fatalf("expected content type audio/wav, got %q", got)
		}
		if rec.Body.Len() == 0 {
			t.Fatal("expected non-empty audio body")
		}
	})

	t.Run("returns internal server error on synthesis failure", func(t *testing.T) {
		h := handlers.NewAIHandlers(
			testutil.NewMockDB(),
			&mockSpeechAIClient{err: context.DeadlineExceeded},
			knowledge.NewEmptyService(),
		)
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", strings.NewReader(`{"highlightedText":"テスト"}`))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()

		h.SpeakAPI(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})

	t.Run("returns too many requests on provider quota failure", func(t *testing.T) {
		h := handlers.NewAIHandlers(
			testutil.NewMockDB(),
			&mockSpeechAIClient{
				err: &ai.ProviderError{Provider: "minimax", Kind: ai.ErrorKindQuota, StatusCode: 429, Message: "quota exceeded"},
			},
			knowledge.NewEmptyService(),
		)
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", strings.NewReader(`{"highlightedText":"テスト"}`))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()

		h.SpeakAPI(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected status 429, got %d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "quota") {
			t.Fatalf("expected quota message, got %s", rec.Body.String())
		}
	})

	t.Run("returns too many requests on analyze provider rate limit", func(t *testing.T) {
		db := testutil.NewMockDB()
		if err := db.CreateUser(context.Background(), &models.User{Email: "test@example.com", Provider: "google", ProviderID: "1"}); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		h := handlers.NewAIHandlers(
			db,
			&mockSpeechAIClient{
				annErr: &ai.ProviderError{Provider: "minimax", Kind: ai.ErrorKindRateLimit, StatusCode: 429, Message: "rate limited"},
			},
			knowledge.NewEmptyService(),
		)
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", strings.NewReader(`{"textToAnalyze":"小川未明","context":"小川未明"}`))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()

		h.AnalyzeAPI(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected status 429, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestRoutedAIHandlerContracts(t *testing.T) {
	for _, status := range []int{200, 401, 402, 429} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			audio, err := os.ReadFile("../ai/testdata/silence.mp3")
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status != 200 {
					w.WriteHeader(status)
					_, _ = w.Write([]byte("private-page fixture-key"))
					return
				}
				if r.URL.Path == "/v1/audio/speech" {
					w.Header().Set("Content-Type", "audio/mpeg")
					_, _ = w.Write(audio)
					return
				}
				annotation := `{"translation":"Japanese","contextual_explanation":"The Japanese language","usage_example":"日本語を話す。","when_to_use":"Speaking about language","word_breakdown":"日本: Japan; 語: language","alternative_meanings":"None","pronunciation":{"kana":"にほんご"}}`
				_ = json.NewEncoder(w).Encode(map[string]any{"model": "test-vision", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": annotation}}}})
			}))
			defer server.Close()
			cfg, err := ai.LoadRoutingConfig("../ai/testdata/routing.json", func(string) string { return "fixture-key" })
			if err != nil {
				t.Fatal(err)
			}
			profile := cfg.Profiles["active"]
			profile.BaseURL = server.URL + "/v1"
			cfg.Profiles["active"] = profile
			client, err := ai.NewRoutedClient(cfg, ai.RoutedClientOptions{})
			if err != nil {
				t.Fatal(err)
			}
			db := testutil.NewMockDB()
			user := &models.User{Email: "test@example.test", Provider: "test", ProviderID: "test"}
			if err := db.CreateUser(context.Background(), user); err != nil {
				t.Fatal(err)
			}
			h := handlers.NewAIHandlers(db, client, knowledge.NewEmptyService())
			for _, speech := range []bool{false, true} {
				body := `{"textToAnalyze":"日本語","context":"private-page"}`
				if speech {
					body = `{"highlightedText":"日本語","contextText":"private-page"}`
				}
				req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", strings.NewReader(body))
				req = req.WithContext(middleware.WithUserID(req.Context(), user.ID))
				rec := httptest.NewRecorder()
				if speech {
					h.SpeakAPI(rec, req)
				} else {
					h.AnalyzeAPI(rec, req)
				}
				want := 200
				if status == 401 {
					want = 502
				}
				if status == 402 || status == 429 {
					want = 429
				}
				if rec.Code != want {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, want, rec.Body.String())
				}
				if status != 200 {
					if strings.Contains(rec.Body.String(), "private-page") || strings.Contains(rec.Body.String(), "fixture-key") {
						t.Fatal("private upstream error exposed")
					}
				} else if speech {
					if rec.Header().Get("Content-Type") != "audio/mpeg" || !bytes.Equal(rec.Body.Bytes(), audio) {
						t.Fatal("speech API contract changed")
					}
				} else {
					var got handlers.AnalyzeResponse
					if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
						t.Fatal(err)
					}
					if got.Translation != "Japanese" || got.Meaning != "The Japanese language" || got.Pronunciation.Kana != "にほんご" {
						t.Fatal("annotation API contract changed")
					}
				}
			}
		})
	}
}
