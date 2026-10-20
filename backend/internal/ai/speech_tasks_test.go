package ai

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSpeechTasksProtocols(t *testing.T) {
	audio, err := os.ReadFile("testdata/silence.mp3")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"openrouter", "modelverse"} {
		t.Run(kind, func(t *testing.T) {
			var event CallMetadata
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("invalid body")
				}
				if string(body["model"]) != `"test-speech"` || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Error("wrong model/auth")
				}
				if bytes.Contains(body["text"], []byte("private-context")) || bytes.Contains(body["input"], []byte("private-context")) {
					t.Error("context spoken")
				}
				if kind == "modelverse" {
					if r.URL.Path != "/v1/t2a_v2" || string(body["text"]) != `"こんにちは"` || string(body["language_boost"]) != `"Japanese"` {
						t.Error("wrong ModelVerse payload")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"audio": hex.EncodeToString(audio), "status": 2}, "base_resp": map[string]any{"status_code": 0}, "extra_info": map[string]any{"audio_format": "mp3", "usage_characters": 5}, "trace_id": "trace-fixture"})
				} else {
					if r.URL.Path != "/v1/audio/speech" || string(body["input"]) != `"こんにちは"` || string(body["voice"]) != `"test-ja"` || string(body["response_format"]) != `"mp3"` {
						t.Error("wrong OpenRouter payload")
					}
					w.Header().Set("Content-Type", "audio/mpeg")
					w.Header().Set("X-Generation-ID", "gen-fixture")
					_, _ = w.Write(audio)
				}
			}))
			defer server.Close()
			g := testGateway(t, server, kind, "speech", RoutedClientOptions{Observe: func(_ context.Context, m CallMetadata) { event = m }})
			got, err := (&speechTask{gateway: g}).SynthesizeSpeech(context.Background(), "こんにちは", "private-context")
			if err != nil {
				t.Fatal(err)
			}
			if got.MIMEType != "audio/mpeg" || !bytes.Equal(got.Audio, audio) {
				t.Fatal("audio corrupted")
			}
			if event.RequestID == "" {
				t.Fatal("missing request ID")
			}
			if kind == "modelverse" && (event.Characters == nil || *event.Characters != 5) {
				t.Fatal("missing billed characters")
			}
			if kind == "openrouter" && event.Characters != nil {
				t.Fatal("fabricated usage")
			}
		})
	}
}

func TestSpeechTasksRejectInvalidAudio(t *testing.T) {
	cases := []struct{ name, kind, mime, body string }{
		{"JSON disguised as audio", "openrouter", "audio/mpeg", `{"error":"private-page fixture-key"}`},
		{"wrong MIME", "openrouter", "application/json", `{"error":"private-page"}`},
		{"empty", "openrouter", "audio/mpeg", ""},
		{"ID3 without audio", "openrouter", "audio/mpeg", "ID3\x04\x00\x00\x00\x00\x00\x00"},
		{"frame header without audio", "openrouter", "audio/mpeg", "\xff\xfb\x90\x00"},
		{"business error", "modelverse", "application/json", `{"base_resp":{"status_code":1004,"status_msg":"fixture-key"}}`},
		{"null data", "modelverse", "application/json", `{"base_resp":{"status_code":0},"data":null}`},
		{"invalid hex", "modelverse", "application/json", `{"base_resp":{"status_code":0},"data":{"status":2,"audio":"xyz"}}`},
		{"incomplete", "modelverse", "application/json", `{"base_resp":{"status_code":0},"data":{"status":1,"audio":"fffb9000"},"extra_info":{"audio_format":"mp3"}}`},
		{"missing business status", "modelverse", "application/json", `{"data":{"status":2,"audio":"fffb9000"},"extra_info":{"audio_format":"mp3"}}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.mime)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			_, err := (&speechTask{gateway: testGateway(t, server, tt.kind, "speech", RoutedClientOptions{})}).SynthesizeSpeech(context.Background(), "こんにちは", "")
			if err == nil {
				t.Fatal("invalid audio accepted")
			}
		})
	}
}
