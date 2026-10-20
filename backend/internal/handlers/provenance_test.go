package handlers

import (
	"context"
	"encoding/json"
	"io"
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
	"github.com/gemini-hackathon/app/internal/usage"
)

func provenanceTestMeter() *usage.Meter {
	return usage.NewMeter(testutil.NewMemoryUsageStore(), usage.Limits{
		usage.FeatureOCR:         {User: 100, Global: 1000},
		usage.FeatureExplanation: {User: 100, Global: 1000},
	})
}

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
			h := NewScanHandlers(db, nil, tt.client, &config.Config{}, provenanceTestMeter())
			h.scans.ProcessOCR(context.Background(), nil, scanID, []byte("image"), "image/png")

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
		NewAIHandlers(db, client, knowledge.NewEmptyService(), provenanceTestMeter()).AnalyzeAPI(rec, req)
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

	// An unapproved reportedModel drops only that field; the configured producer stays.
	t.Run("request id as reported model", func(t *testing.T) {
		body := analyze(reporting)
		body["provenance"] = json.RawMessage(`{"version":1,"provider":"routed","gateway":"modelverse","region":"singapore","model":"MiniMax-M3","reportedModel":"req-123","promptVersion":"annotation/1"}`)
		if saved := save(reporting, body); saved.NuanceData.Provenance == nil || *saved.NuanceData.Provenance != want {
			t.Fatalf("saved provenance = %+v, want configured producer only", saved.NuanceData.Provenance)
		}
	})

	t.Run("handler without AI client stores no provenance", func(t *testing.T) {
		if saved := save(nil, analyze(reporting)); saved.NuanceData.Provenance != nil {
			t.Fatalf("saved provenance = %+v, want unknown", saved.NuanceData.Provenance)
		}
	})
}

// routedProvenanceClient is a real routed client against a local gateway that
// reports reportedModel; Observe feeds the request-scoped usage collector
// exactly as main.go does.
func routedProvenanceClient(t *testing.T, reportedModel string) ai.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{"translation":"library","contextual_explanation":"A place for books","usage_example":"図書館で読む。","when_to_use":"Places","word_breakdown":"図書: books; 館: building","alternative_meanings":"None","pronunciation":{"kana":"としょかん"}}`
		if strings.Contains(readBody(t, r), "raw_text") {
			content = `{"raw_text":"駅前の図書館","language":"ja"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": reportedModel, "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": content}}}})
	}))
	t.Cleanup(server.Close)
	cfg, err := ai.LoadRoutingConfig("../ai/testdata/routing.json", func(string) string { return "fixture-key" })
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["active"]
	profile.BaseURL = server.URL + "/v1"
	model := profile.Models["test-vision"]
	model.ReportedModelAliases = []string{"test-vision-20261020"}
	profile.Models["test-vision"] = model
	cfg.Profiles["active"] = profile
	client, err := ai.NewRoutedClient(cfg, ai.RoutedClientOptions{Observe: usage.RecordCall})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	var body strings.Builder
	if _, err := io.Copy(&body, r.Body); err != nil {
		t.Fatal(err)
	}
	return body.String()
}

func TestRoutedProvenanceRecordsReportedModel(t *testing.T) {

	configured := models.AIProvenance{Version: 1, Provider: "routed", Gateway: "openrouter", Region: "global", Model: "test-vision"}

	t.Run("ocr stores the alias the gateway reported", func(t *testing.T) {
		client := routedProvenanceClient(t, "test-vision-20261020")
		db := testutil.NewMockDB()
		scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing"})
		if err != nil {
			t.Fatal(err)
		}
		meter := provenanceTestMeter()
		ctx, reservation, err := meter.Reserve(context.Background(), 1, usage.FeatureOCR, 1)
		if err != nil {
			t.Fatal(err)
		}
		png := []byte("\x89PNG\r\n\x1a\n")
		NewScanHandlers(db, nil, client, &config.Config{}, meter).scans.ProcessOCR(ctx, reservation, scanID, png, "image/png")
		got, _ := db.ScanOCRProvenance(scanID)
		want := configured
		want.PromptVersion, want.ReportedModel = ai.PromptVersionOCRRoutedJSON, "test-vision-20261020"
		if got == nil || *got != want {
			t.Fatalf("provenance = %+v, want %+v", got, want)
		}
	})

	t.Run("ocr without a request collector keeps reported model unknown", func(t *testing.T) {
		client := routedProvenanceClient(t, "test-vision-20261020")
		db := testutil.NewMockDB()
		scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, Status: "processing"})
		if err != nil {
			t.Fatal(err)
		}
		NewScanHandlers(db, nil, client, &config.Config{}, provenanceTestMeter()).scans.ProcessOCR(context.Background(), nil, scanID, []byte("\x89PNG\r\n\x1a\n"), "image/png")
		got, _ := db.ScanOCRProvenance(scanID)
		if got == nil || got.ReportedModel != "" || got.Model != "test-vision" {
			t.Fatalf("provenance = %+v, want configured producer with unknown reported model", got)
		}
	})

	t.Run("analyze reports the model and save keeps only approved values", func(t *testing.T) {
		client := routedProvenanceClient(t, "test-vision-20261020")
		db := testutil.NewMockDB()
		if err := db.CreateUser(context.Background(), &models.User{Email: "r@example.com", Provider: "clerk", ProviderID: "u1"}); err != nil {
			t.Fatal(err)
		}
		scanID, err := db.CreateScan(context.Background(), &models.Scan{UserID: 1, SourceType: "image", Status: "ready"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/analyze", strings.NewReader(`{"textToAnalyze":"図書館","context":"駅前の図書館"}`))
		req = req.WithContext(middleware.WithUserID(req.Context(), 1))
		rec := httptest.NewRecorder()
		NewAIHandlers(db, client, knowledge.NewEmptyService(), provenanceTestMeter()).AnalyzeAPI(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("analyze status %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		want := configured
		want.PromptVersion, want.ReportedModel = ai.PromptVersionAnnotation, "test-vision-20261020"
		var analyzed models.AIProvenance
		if err := json.Unmarshal(body["provenance"], &analyzed); err != nil || analyzed != want {
			t.Fatalf("analyze provenance = %+v (%v), want %+v", analyzed, err, want)
		}

		saveWith := func(reported string) *models.AIProvenance {
			echoed := want
			echoed.ReportedModel = reported
			raw, err := json.Marshal(echoed)
			if err != nil {
				t.Fatal(err)
			}
			body["provenance"] = raw
			payload, err := json.Marshal(map[string]any{"scanId": scanID, "highlightedText": "図書館", "nuanceData": body})
			if err != nil {
				t.Fatal(err)
			}
			saveReq := httptest.NewRequest(http.MethodPost, "/v1/annotations", strings.NewReader(string(payload)))
			saveReq = saveReq.WithContext(middleware.WithUserID(saveReq.Context(), 1))
			saveRec := httptest.NewRecorder()
			NewAnnotationHandlers(db, &config.Config{}).WithAIClient(client).CreateAnnotationAPI(saveRec, saveReq)
			var created struct {
				AnnotationID int64 `json:"annotationId"`
			}
			if err := json.Unmarshal(saveRec.Body.Bytes(), &created); err != nil {
				t.Fatalf("save failed (%d): %s", saveRec.Code, saveRec.Body.String())
			}
			annotation, _ := db.GetAnnotationByID(context.Background(), created.AnnotationID)
			return annotation.NuanceData.Provenance
		}
		// The server cannot bind an echoed reportedModel to the saved output, so
		// every echo, including the genuine alias, a swapped selected model, an
		// invented value or none at all, saves the configured producer only.
		configuredOnly := want
		configuredOnly.ReportedModel = ""
		for _, echoed := range []string{"test-vision-20261020", "test-vision", "", "req-123", "sk-test-secret", "other-model"} {
			if got := saveWith(echoed); got == nil || *got != configuredOnly {
				t.Fatalf("echoed reported model %q: saved %+v, want configured producer only", echoed, got)
			}
		}
	})
}
