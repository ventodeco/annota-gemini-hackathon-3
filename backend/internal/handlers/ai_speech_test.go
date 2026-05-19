package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
