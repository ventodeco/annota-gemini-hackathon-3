package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type contextCapturingAIClient struct {
	provenanceAIClient
	context string
}

func (c *contextCapturingAIClient) AnnotateWithKnowledge(_ context.Context, contextText, _ string, _ []knowledge.Entry) (*ai.AnnotationResponse, error) {
	c.context = contextText
	return &ai.AnnotationResponse{Translation: "library"}, nil
}

// A scanId makes analyze use the stored page text, windowed around the
// selection, instead of whatever context the client sent.
func TestAnalyzeLoadsServerScanContext(t *testing.T) {
	db := testutil.NewMockDB()
	if err := db.CreateUser(context.Background(), &models.User{Email: "c@example.com", Provider: "clerk", ProviderID: "u1"}); err != nil {
		t.Fatal(err)
	}
	pageText := strings.Repeat("あ", 20000) + "図書館" + strings.Repeat("い", 20000)
	scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, SourceType: "pdf", Status: "ready", FullOCRText: &pageText})
	if err != nil {
		t.Fatal(err)
	}

	client := &contextCapturingAIClient{}
	body := fmt.Sprintf(`{"textToAnalyze":"図書館","context":"client context","scanId":%d}`, scanID)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", strings.NewReader(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	NewAIHandlers(db, client, knowledge.NewEmptyService(), provenanceTestMeter()).AnalyzeAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("analyze status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(client.context, "図書館") || strings.Contains(client.context, "client context") {
		t.Fatalf("expected server page text around the selection, got %d runes", utf8.RuneCountInString(client.context))
	}
	if got := utf8.RuneCountInString(client.context); got != 12000 {
		t.Fatalf("expected a 12000-rune window, got %d", got)
	}
}
