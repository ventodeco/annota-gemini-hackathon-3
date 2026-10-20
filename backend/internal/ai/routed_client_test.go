package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func TestRoutedClientSeparatesTasksAndFreezesRoutes(t *testing.T) {
	var ocrCalls, annotationCalls, speechCalls atomic.Int32
	makeServer := func(counter *atomic.Int32, key string, text bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			if r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("cross-profile credential leak")
			}
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Model != "test-vision" && body.Model != "test-speech" {
				t.Error("route changed after construction")
			}
			if !text {
				audio, _ := os.ReadFile("testdata/silence.mp3")
				w.Header().Set("Content-Type", "audio/mpeg")
				_, _ = w.Write(audio)
				return
			}
			result := annotationFixture
			if counter == &ocrCalls {
				result = `{"raw_text":"日本語","language":"ja"}`
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"model": body.Model, "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": result}}}})
		}))
	}
	a := makeServer(&ocrCalls, "ocr-key", true)
	defer a.Close()
	b := makeServer(&annotationCalls, "annotation-key", true)
	defer b.Close()
	s := makeServer(&speechCalls, "speech-key", false)
	defer s.Close()
	cfg := routingFixture(t)
	for id, server := range map[string]*httptest.Server{"ocr": a, "annotation": b, "speech": s} {
		p := cfg.Profiles["active"]
		p.APIKey = id + "-key"
		p.BaseURL = server.URL + "/v1"
		cfg.Profiles[id] = p
	}
	cfg.Tasks.OCR.Profile = "ocr"
	cfg.Tasks.Annotation.Profile = "annotation"
	cfg.Tasks.Speech.Profile = "speech"
	client, err := NewRoutedClient(cfg, RoutedClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Tasks.OCR.Model = "unapproved"
	if _, err = client.OCR(context.Background(), []byte("page"), "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Annotate(context.Background(), "page", "日本語"); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SynthesizeSpeech(context.Background(), "日本語", ""); err != nil {
		t.Fatal(err)
	}
	if ocrCalls.Load() != 1 || annotationCalls.Load() != 1 || speechCalls.Load() != 1 {
		t.Fatal("wrong task routes")
	}
	if _, err = NewRoutedClient(cfg, RoutedClientOptions{}); err == nil {
		t.Fatal("invalid route accepted")
	}
}
