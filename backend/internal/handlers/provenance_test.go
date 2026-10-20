package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/testutil"
)

type provenanceAIClient struct {
	producers map[string]ai.Producer
}

func (c *provenanceAIClient) OCR(context.Context, []byte, string) (*ai.OCRResponse, error) {
	return &ai.OCRResponse{RawText: "駅前の図書館", Language: "ja"}, nil
}

func (c *provenanceAIClient) Annotate(context.Context, string, string) (*ai.AnnotationResponse, error) {
	return nil, nil
}

func (c *provenanceAIClient) AnnotateWithKnowledge(context.Context, string, string, []knowledge.Entry) (*ai.AnnotationResponse, error) {
	return &ai.AnnotationResponse{Translation: "library", Pronunciation: ai.Pronunciation{Kana: "としょかん"}}, nil
}

func (c *provenanceAIClient) SynthesizeSpeech(context.Context, string, string) (*ai.SpeechResponse, error) {
	return nil, nil
}

// unreportedAIClient exposes only ai.Client, hiding the producer like test doubles do.
type unreportedAIClient struct{ ai.Client }

func (c *provenanceAIClient) Producer(task string) (ai.Producer, bool) {
	producer, ok := c.producers[task]
	return producer, ok
}

var routedProducers = map[string]ai.Producer{
	"ocr":        {Provider: ai.ProviderRouted, Gateway: "openrouter", Region: "global", Model: "google/gemini-3.5-flash-lite", PromptVersion: ai.PromptVersionOCRRoutedJSON},
	"annotation": {Provider: ai.ProviderRouted, Gateway: "modelverse", Region: "singapore", Model: "MiniMax-M3", PromptVersion: ai.PromptVersionAnnotation},
}

func TestProcessOCRStoresProducerProvenance(t *testing.T) {
	tests := []struct {
		name   string
		client ai.Client
		want   *models.AIProvenance
	}{
		{"reporting client", &provenanceAIClient{producers: routedProducers}, &models.AIProvenance{Version: 1, Provider: "routed", Gateway: "openrouter", Region: "global", Model: "google/gemini-3.5-flash-lite", PromptVersion: ai.PromptVersionOCRRoutedJSON}},
		{"unreported producer stays unknown", &unreportedAIClient{Client: &provenanceAIClient{producers: routedProducers}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutil.NewMockDB()
			scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing"})
			if err != nil {
				t.Fatal(err)
			}
			h := NewScanHandlers(db, nil, tt.client, &config.Config{})
			h.processOCR(context.Background(), scanID, []byte("image"), "image/png", "scan.png")

			got, recorded := db.ScanOCRProvenance(scanID)
			if !recorded {
				t.Fatal("OCR result was not saved")
			}
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("provenance = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeProvenanceRoundTripsIntoSavedAnnotation(t *testing.T) {
	db := testutil.NewMockDB()
	if err := db.CreateUser(context.Background(), &models.User{Email: "p@example.com", Provider: "clerk", ProviderID: "u1"}); err != nil {
		t.Fatal(err)
	}
	scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, SourceType: "image", Status: "ready"})
	if err != nil {
		t.Fatal(err)
	}

	analyze := func(client ai.Client) map[string]json.RawMessage {
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", strings.NewReader(`{"textToAnalyze":"図書館","context":"駅前の図書館"}`))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()
		NewAIHandlers(db, client, knowledge.NewEmptyService()).AnalyzeAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("analyze status %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	save := func(client ai.Client, nuanceData map[string]json.RawMessage) *models.Annotation {
		payload, err := json.Marshal(map[string]any{"scanId": scanID, "highlightedText": "図書館", "contextText": "駅前の図書館", "nuanceData": nuanceData})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/annotations", strings.NewReader(string(payload)))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()
		NewAnnotationHandlers(db, &config.Config{}).WithAIClient(client).CreateAnnotationAPI(rec, req)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("save status %d: %s", rec.Code, rec.Body.String())
		}
		var created struct {
			AnnotationID int64 `json:"annotationId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		annotation, _ := db.GetAnnotationByID(context.Background(), created.AnnotationID)
		if annotation == nil {
			t.Fatal("annotation not saved")
		}
		return annotation
	}

	reporting := &provenanceAIClient{producers: routedProducers}
	want := models.AIProvenance{Version: 1, Provider: "routed", Gateway: "modelverse", Region: "singapore", Model: "MiniMax-M3", PromptVersion: ai.PromptVersionAnnotation}

	t.Run("reporting client provenance is saved with the nuance data", func(t *testing.T) {
		saved := save(reporting, analyze(reporting))
		if saved.NuanceData.Provenance == nil || *saved.NuanceData.Provenance != want {
			t.Fatalf("saved provenance = %+v, want %+v", saved.NuanceData.Provenance, want)
		}
		if saved.NuanceData.Translation != "library" || saved.NuanceData.Pronunciation.Kana != "としょかん" {
			t.Fatalf("nuance data not saved: %+v", saved.NuanceData)
		}
	})

	t.Run("unreported producer has no provenance", func(t *testing.T) {
		unreported := &unreportedAIClient{Client: reporting}
		body := analyze(unreported)
		if _, ok := body["provenance"]; ok {
			t.Fatalf("unexpected provenance in analyze response: %s", body["provenance"])
		}
		if saved := save(unreported, body); saved.NuanceData.Provenance != nil {
			t.Fatalf("saved provenance = %+v, want unknown", saved.NuanceData.Provenance)
		}
	})

	// Echoed values that do not exactly match the server's producer are dropped,
	// never stored and never a reason to reject the save.
	forged := map[string]string{
		"token-shaped secret as model":   `{"version":1,"provider":"routed","gateway":"modelverse","region":"singapore","model":"sk-test-secret","promptVersion":"annotation/1"}`,
		"request id as reported model":   `{"version":1,"provider":"routed","gateway":"modelverse","region":"singapore","model":"MiniMax-M3","reportedModel":"req-123","promptVersion":"annotation/1"}`,
		"source text as model":           `{"version":1,"provider":"routed","model":"駅前の図書館","promptVersion":"annotation/1"}`,
		"wrong field type":               `{"version":"1"}`,
		"not an object":                  `"routed"`,
		"producer changed before saving": `{"version":1,"provider":"routed","gateway":"openrouter","region":"global","model":"google/gemini-3.5-flash-lite","promptVersion":"annotation/1"}`,
	}
	for name, value := range forged {
		t.Run(name, func(t *testing.T) {
			body := analyze(reporting)
			body["provenance"] = json.RawMessage(value)
			saved := save(reporting, body)
			if saved.NuanceData.Provenance != nil {
				t.Fatalf("saved provenance = %+v, want dropped", saved.NuanceData.Provenance)
			}
			if saved.NuanceData.Translation != "library" {
				t.Fatalf("nuance data not saved: %+v", saved.NuanceData)
			}
		})
	}

	t.Run("handler without AI client stores no provenance", func(t *testing.T) {
		if saved := save(nil, analyze(reporting)); saved.NuanceData.Provenance != nil {
			t.Fatalf("saved provenance = %+v, want unknown", saved.NuanceData.Provenance)
		}
	})
}
