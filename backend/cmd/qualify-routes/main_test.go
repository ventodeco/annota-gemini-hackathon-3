package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	contextText   = "秘密の本文です。これは検証用の文章です。"
	selectionText = "検証用"
	ocrText       = "非公開の転写テキスト"
	privateNote   = "privatetranslationsentinel"
)

var annotationContent = `{"translation":"` + privateNote + `","contextual_explanation":"Context sentinel","usage_example":"検証用の例文。","when_to_use":"Testing","word_breakdown":{"検証":"verification"},"alternative_meanings":"None","pronunciation":{"kana":"けんしょうよう"}}`

type fixture struct {
	ocr, annotation, speech *httptest.Server
	ocrCalls                atomic.Int32
	annotationCalls         atomic.Int32
	speechCalls             atomic.Int32
	ocrID, ocrLanguage      string
	ocrRaw                  string
	annotationStatus        int
	configPath, imagePath   string
	contextPath, dir        string
	env                     map[string]string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{ocrID: "gen-ocr-1", ocrLanguage: "ja", ocrRaw: ocrText, annotationStatus: http.StatusOK, dir: t.TempDir()}
	f.env = map[string]string{"QUALIFY_OCR_KEY": "ocr-secret-value", "QUALIFY_ANNOTATION_KEY": "annotation-secret-value", "QUALIFY_SPEECH_KEY": "speech-secret-value"}
	chat := func(calls *atomic.Int32, key string, content func() (string, string, int)) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("wrong credential for route")
			}
			text, id, status := content()
			if status != http.StatusOK {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"message":"` + contextText + `"}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "model": "test-vision", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": text}}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20}})
		}))
	}
	f.ocr = chat(&f.ocrCalls, f.env["QUALIFY_OCR_KEY"], func() (string, string, int) {
		return `{"raw_text":"` + f.ocrRaw + `","language":"` + f.ocrLanguage + `"}`, f.ocrID, http.StatusOK
	})
	f.annotation = chat(&f.annotationCalls, f.env["QUALIFY_ANNOTATION_KEY"], func() (string, string, int) {
		return annotationContent, "gen-annotation-1", f.annotationStatus
	})
	audio, err := os.ReadFile("../../internal/ai/testdata/silence.mp3")
	if err != nil {
		t.Fatal(err)
	}
	f.speech = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.speechCalls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+f.env["QUALIFY_SPEECH_KEY"] {
			t.Error("wrong credential for speech route")
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("X-Request-ID", "req-speech-1")
		_, _ = w.Write(audio)
	}))
	t.Cleanup(func() { f.ocr.Close(); f.annotation.Close(); f.speech.Close() })

	chatModel := map[string]any{"image_input": true, "text_input": true, "text_output": true, "json_mode": "json_schema", "ocr_output": "json", "max_output_tokens": 2048}
	speechModel := map[string]any{"text_input": true, "audio_output": true, "japanese_voices": []string{"test-ja"}, "audio_formats": []string{"mp3"}}
	profile := func(server *httptest.Server, keyEnv string, models map[string]any) map[string]any {
		return map[string]any{"kind": "openrouter", "region": "global", "base_url": server.URL + "/v1", "api_key_env": keyEnv, "timeout_seconds": 5, "max_attempts": 1, "models": models}
	}
	cfg := map[string]any{
		"version": 1,
		"profiles": map[string]any{
			"ocr":        profile(f.ocr, "QUALIFY_OCR_KEY", map[string]any{"test-vision": chatModel}),
			"annotation": profile(f.annotation, "QUALIFY_ANNOTATION_KEY", map[string]any{"test-vision": chatModel}),
			"speech":     profile(f.speech, "QUALIFY_SPEECH_KEY", map[string]any{"test-speech": speechModel}),
		},
		"tasks": map[string]any{
			"ocr":        map[string]any{"profile": "ocr", "model": "test-vision"},
			"annotation": map[string]any{"profile": "annotation", "model": "test-vision"},
			"speech":     map[string]any{"profile": "speech", "model": "test-speech", "voice": "test-ja", "format": "mp3"},
		},
	}
	f.configPath = writeJSON(t, f.dir, "routing.json", cfg)
	f.contextPath = filepath.Join(f.dir, "context.txt")
	if err := os.WriteFile(f.contextPath, []byte(contextText), 0o600); err != nil {
		t.Fatal(err)
	}
	var page bytes.Buffer
	if err := png.Encode(&page, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	f.imagePath = filepath.Join(f.dir, "page.png")
	if err := os.WriteFile(f.imagePath, page.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func writeJSON(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f *fixture) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), append([]string{"-config", f.configPath}, args...), func(name string) string { return f.env[name] }, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func (f *fixture) liveArgs(extra ...string) []string {
	return append([]string{"-live", "-image", f.imagePath, "-context-file", f.contextPath, "-selection", selectionText}, extra...)
}

func (f *fixture) calls() [3]int32 {
	return [3]int32{f.ocrCalls.Load(), f.annotationCalls.Load(), f.speechCalls.Load()}
}

func assertPrivate(t *testing.T, f *fixture, outputs ...string) {
	t.Helper()
	private := []string{contextText, selectionText, ocrText, privateNote, "Context sentinel", "けんしょうよう", f.configPath}
	for _, value := range f.env {
		private = append(private, value)
	}
	for _, output := range outputs {
		for _, value := range private {
			if strings.Contains(output, value) {
				t.Fatalf("output leaked private value %q", value)
			}
		}
	}
}

func decodeReport(t *testing.T, stdout string) qualificationReport {
	t.Helper()
	var report qualificationReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}
	return report
}

func taskResult(t *testing.T, report qualificationReport, task string) taskReport {
	t.Helper()
	for _, result := range report.Tasks {
		if result.Task == task {
			return result
		}
	}
	t.Fatalf("missing %s result", task)
	return taskReport{}
}

func TestValidateOnlyMakesNoRequests(t *testing.T) {
	f := newFixture(t)
	code, stdout, stderr := f.run(t)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if f.calls() != [3]int32{} {
		t.Fatalf("validate-only made requests: %v", f.calls())
	}
	report := decodeReport(t, stdout)
	if report.Live || len(report.Tasks) != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
	speech := taskResult(t, report, "speech")
	if speech.Status != "validated" || speech.Profile != "speech" || speech.Gateway != "openrouter" || speech.Model != "test-speech" || speech.Voice != "test-ja" || speech.Format != "mp3" {
		t.Fatalf("unexpected speech route: %+v", speech)
	}
	assertPrivate(t, f, stdout, stderr)
}

func TestLiveRunsOneBoundedCallPerTaskWithoutContent(t *testing.T) {
	f := newFixture(t)
	audioPath := filepath.Join(f.dir, "speech.mp3")
	code, stdout, stderr := f.run(t, f.liveArgs("-audio-out", audioPath)...)
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stderr, stdout)
	}
	if f.calls() != [3]int32{1, 1, 1} {
		t.Fatalf("expected one call per task, got %v", f.calls())
	}
	report := decodeReport(t, stdout)
	ocr := taskResult(t, report, "ocr")
	if ocr.Status != "passed" || len(ocr.Attempts) != 1 || ocr.Attempts[0].RequestID != "gen-ocr-1" || ocr.Attempts[0].InputTokens == nil {
		t.Fatalf("unexpected OCR result: %+v", ocr)
	}
	if ocr.Checks["contains_japanese"] != true || ocr.Checks["text_runes"] != float64(len([]rune(ocrText))) || ocr.Checks["language_is_japanese"] != true {
		t.Fatalf("unexpected OCR checks: %+v", ocr.Checks)
	}
	annotation := taskResult(t, report, "annotation")
	if annotation.Status != "passed" || annotation.Checks["required_fields_present"] != true {
		t.Fatalf("unexpected annotation result: %+v", annotation)
	}
	speech := taskResult(t, report, "speech")
	if speech.Status != "passed" || speech.Checks["mime_type"] != "audio/mpeg" || speech.Checks["mp3_header"] != true || speech.Attempts[0].RequestID != "req-speech-1" {
		t.Fatalf("unexpected speech result: %+v", speech)
	}
	audio, err := os.ReadFile(audioPath)
	if err != nil || len(audio) == 0 || speech.Checks["audio_bytes"] != float64(len(audio)) {
		t.Fatalf("audio not written: %v", err)
	}
	if info, err := os.Stat(audioPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("audio file permissions: %v", err)
	}
	assertPrivate(t, f, stdout, stderr)
}

func TestLiveFlagsAndRedactsEchoedRequestID(t *testing.T) {
	f := newFixture(t)
	f.ocrID = "gen-本文です"
	code, stdout, stderr := f.run(t, f.liveArgs("-tasks", "ocr")...)
	if code != 1 {
		t.Fatalf("echoed ID must fail qualification, exit %d", code)
	}
	ocr := taskResult(t, decodeReport(t, stdout), "ocr")
	if !ocr.MetadataEchoesContent || ocr.Attempts[0].RequestID != redactedID {
		t.Fatalf("echo not flagged/redacted: %+v", ocr)
	}
	if strings.Contains(stdout+stderr, "本文です") {
		t.Fatal("echoed fragment printed")
	}
	assertPrivate(t, f, stdout, stderr)
}

func TestLiveNeverReportsOCRLanguageText(t *testing.T) {
	for _, language := range []string{"Japanese: 本文です", "confidential", "ja"} {
		t.Run(language, func(t *testing.T) {
			f := newFixture(t)
			f.ocrLanguage = language
			code, stdout, stderr := f.run(t, f.liveArgs("-tasks", "ocr")...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			checks := taskResult(t, decodeReport(t, stdout), "ocr").Checks
			if _, ok := checks["language"]; ok || (language != "ja" && strings.Contains(stdout, language)) {
				t.Fatal("model language text reported")
			}
			if checks["language_is_japanese"] != (language == "ja") {
				t.Fatalf("unexpected language check: %v", checks["language_is_japanese"])
			}
		})
	}
}

func TestLiveFlagsShortJapaneseEchoes(t *testing.T) {
	tests := []struct{ name, raw, id string }{
		{"two-rune output", "秘密", "gen-秘密"},
		{"one-rune selection", ocrText, "gen-検"},
		{"unrelated Japanese", ocrText, "gen-猫"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.ocrRaw, f.ocrID = tt.raw, tt.id
			code, stdout, _ := f.run(t, f.liveArgs("-tasks", "ocr")...)
			ocr := taskResult(t, decodeReport(t, stdout), "ocr")
			if code != 1 || !ocr.MetadataEchoesContent || ocr.Attempts[0].RequestID != redactedID || strings.Contains(stdout, tt.id) {
				t.Fatalf("short echo not redacted: exit %d %+v", code, ocr)
			}
		})
	}
}

func TestLiveFlagsCompleteShortASCIIOutput(t *testing.T) {
	f := newFixture(t)
	f.ocrRaw, f.ocrID = "Secret", "gen-secret-1"
	code, stdout, _ := f.run(t, f.liveArgs("-tasks", "ocr")...)
	if ocr := taskResult(t, decodeReport(t, stdout), "ocr"); code != 1 || !ocr.MetadataEchoesContent {
		t.Fatalf("complete ASCII output echo not flagged: exit %d %+v", code, ocr)
	}
}

func TestLiveRedactsUnsafeProviderIDs(t *testing.T) {
	tests := []struct{ name, raw, language, id string }{
		{"URL-shaped ID", ocrText, "ja", "https://private.example/doc/42"},
		{"path-shaped ID", ocrText, "ja", "gen/doc/42"},
		{"non-ASCII ID that lowercases to ASCII", ocrText, "ja", "gen-\u212a-1"},
		{"short OCR language", ocrText, "Secret", "gen-secret-1"},
		{"short word inside output", "Top merit plan", "ja", "gen-merit-7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.ocrRaw, f.ocrLanguage, f.ocrID = tt.raw, tt.language, tt.id
			code, stdout, _ := f.run(t, f.liveArgs("-tasks", "ocr")...)
			ocr := taskResult(t, decodeReport(t, stdout), "ocr")
			if code != 1 || !ocr.MetadataEchoesContent || ocr.Attempts[0].RequestID != redactedID || strings.Contains(stdout, tt.id) {
				t.Fatalf("unsafe ID not redacted: exit %d %+v", code, ocr)
			}
		})
	}
}

func TestInvalidFlagValuesAreNotEchoed(t *testing.T) {
	f := newFixture(t)
	for _, args := range [][]string{{"-live=秘密の本文"}, {"-tasks", "ocr,秘密の本文"}, {"-秘密の本文"}} {
		code, stdout, stderr := f.run(t, args...)
		if code != 2 || stdout != "" || stderr == "" || strings.Contains(stderr, "秘密の本文") {
			t.Fatalf("args %q: exit %d stderr %q", args, code, stderr)
		}
	}
	if f.calls() != [3]int32{} {
		t.Fatal("invalid flags made requests")
	}
}

func TestLiveReportsProviderErrorsAndContinues(t *testing.T) {
	f := newFixture(t)
	f.annotationStatus = http.StatusPaymentRequired
	code, stdout, stderr := f.run(t, f.liveArgs()...)
	if code != 1 {
		t.Fatalf("provider failure must fail qualification, exit %d", code)
	}
	if f.calls() != [3]int32{1, 1, 1} {
		t.Fatalf("other tasks must still run once: %v", f.calls())
	}
	annotation := taskResult(t, decodeReport(t, stdout), "annotation")
	if annotation.Status != "failed" || annotation.ErrorKind != "quota" || annotation.HTTPStatus != http.StatusPaymentRequired {
		t.Fatalf("unexpected annotation failure: %+v", annotation)
	}
	assertPrivate(t, f, stdout, stderr)
}

func TestTaskSelectionLimitsInputsAndCalls(t *testing.T) {
	f := newFixture(t)
	code, stdout, stderr := f.run(t, "-live", "-tasks", "speech", "-selection", selectionText)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if f.calls() != [3]int32{0, 0, 1} {
		t.Fatalf("only speech should be called: %v", f.calls())
	}
	if report := decodeReport(t, stdout); len(report.Tasks) != 1 {
		t.Fatalf("unexpected tasks: %+v", report.Tasks)
	}
}

func TestUsageAndConfigErrorsMakeNoRequests(t *testing.T) {
	f := newFixture(t)
	existing := filepath.Join(f.dir, "existing.mp3")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkDir := filepath.Join(f.dir, "linked-output")
	if err := os.Symlink(f.dir, symlinkDir); err != nil {
		t.Fatal(err)
	}
	unusedDir := filepath.Join(f.dir, "validate-only-output")
	textPath := filepath.Join(f.dir, "not-image.png")
	if err := os.WriteFile(textPath, []byte(contextText), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		env  func()
	}{
		{"missing image", []string{"-live", "-context-file", f.contextPath, "-selection", selectionText}, nil},
		{"selection outside context", []string{"-live", "-image", f.imagePath, "-context-file", f.contextPath, "-selection", "不在"}, nil},
		{"unsupported image", []string{"-live", "-image", textPath, "-context-file", f.contextPath, "-selection", selectionText}, nil},
		{"unknown task", []string{"-tasks", "ocr,video"}, nil},
		{"existing audio output", f.liveArgs("-audio-out", existing), nil},
		{"output dir without live", []string{"-output-dir", unusedDir}, nil},
		{"existing output dir", f.liveArgs("-output-dir", f.dir), nil},
		{"symlinked output dir", f.liveArgs("-output-dir", symlinkDir), nil},
		{"output dir without parent", f.liveArgs("-output-dir", filepath.Join(f.dir, "missing", "out")), nil},
		{"missing active key", nil, func() { delete(f.env, "QUALIFY_SPEECH_KEY") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != nil {
				tt.env()
			}
			code, stdout, stderr := f.run(t, tt.args...)
			if code != 2 || stdout != "" || stderr == "" {
				t.Fatalf("expected usage/config failure, exit %d stdout %q", code, stdout)
			}
			assertPrivate(t, f, stdout, stderr)
		})
	}
	if f.calls() != [3]int32{} {
		t.Fatalf("invalid input made requests: %v", f.calls())
	}
	if data, _ := os.ReadFile(existing); string(data) != "keep" {
		t.Fatal("existing audio output overwritten")
	}
	if _, err := os.Lstat(unusedDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("validate-only run created an output directory")
	}
}

func TestLiveOutputDirStoresOutputOutsideReport(t *testing.T) {
	f := newFixture(t)
	outputDir := filepath.Join(f.dir, "private-output")
	code, stdout, stderr := f.run(t, f.liveArgs("-output-dir", outputDir)...)
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stderr, stdout)
	}
	if f.calls() != [3]int32{1, 1, 1} {
		t.Fatalf("expected one call per task, got %v", f.calls())
	}
	if info, err := os.Stat(outputDir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("output dir permissions: %v", err)
	}
	ocrOut, err := os.ReadFile(filepath.Join(outputDir, "ocr.txt"))
	if err != nil || string(ocrOut) != ocrText {
		t.Fatalf("ocr output = %q, %v", ocrOut, err)
	}
	annotationOut, err := os.ReadFile(filepath.Join(outputDir, "annotation.json"))
	if err != nil || !strings.Contains(string(annotationOut), privateNote) || !strings.Contains(string(annotationOut), "けんしょうよう") {
		t.Fatalf("annotation output = %q, %v", annotationOut, err)
	}
	for _, name := range []string{"ocr.txt", "annotation.json"} {
		if info, err := os.Stat(filepath.Join(outputDir, name)); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions: %v", name, err)
		}
	}
	report := decodeReport(t, stdout)
	for _, task := range []string{"ocr", "annotation"} {
		if result := taskResult(t, report, task); result.Status != "passed" || result.Checks["output_written"] != true {
			t.Fatalf("unexpected %s result: %+v", task, result)
		}
	}
	if strings.Contains(stdout+stderr, outputDir) {
		t.Fatal("report echoed the output directory")
	}
	assertPrivate(t, f, stdout, stderr)
}

func TestWriteOutputFailureIsFixedAndRecorded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ocr.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := taskReport{Checks: map[string]any{}}
	err := writeOutput(dir, "ocr.txt", []byte(ocrText), &result)
	if err == nil || err.Error() != "cannot create -output-dir file" || result.Checks["output_written"] != false {
		t.Fatalf("err = %v, checks = %+v", err, result.Checks)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "ocr.txt")); string(data) != "keep" {
		t.Fatal("existing output overwritten")
	}
	if err := writeOutput("", "ocr.txt", []byte(ocrText), &result); err != nil {
		t.Fatalf("no output dir must be a no-op: %v", err)
	}
}
