package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/gemini"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type mockSpeechGeminiClient struct {
	resp *gemini.SpeechResponse
	err  error
}

func (m *mockSpeechGeminiClient) OCR(ctx context.Context, imageData []byte, mimeType string) (*gemini.OCRResponse, error) {
	return nil, nil
}

func (m *mockSpeechGeminiClient) Annotate(ctx context.Context, ocrText string, selectedText string) (*gemini.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockSpeechGeminiClient) AnnotateWithKnowledge(ctx context.Context, ocrText string, selectedText string, entries []knowledge.Entry) (*gemini.AnnotationResponse, error) {
	return nil, nil
}

func (m *mockSpeechGeminiClient) SynthesizeSpeech(ctx context.Context, highlightedText string, contextText string) (*gemini.SpeechResponse, error) {
	return m.resp, m.err
}

func TestSpeakAPI(t *testing.T) {
	t.Run("returns unauthorized when user is missing", func(t *testing.T) {
		h := handlers.NewAIHandlers(testutil.NewMockDB(), &mockSpeechGeminiClient{}, knowledge.NewEmptyService())
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/speech", strings.NewReader(`{"highlightedText":"テスト"}`))
		rec := httptest.NewRecorder()

		h.SpeakAPI(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("returns bad request when highlighted text is empty", func(t *testing.T) {
		h := handlers.NewAIHandlers(testutil.NewMockDB(), &mockSpeechGeminiClient{}, knowledge.NewEmptyService())
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
			&mockSpeechGeminiClient{
				resp: &gemini.SpeechResponse{
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
			&mockSpeechGeminiClient{err: context.DeadlineExceeded},
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
}
