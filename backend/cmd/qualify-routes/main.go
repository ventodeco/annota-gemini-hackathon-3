// Command qualify-routes validates an AI routing file and, with -live, makes one
// bounded OCR, annotation and speech call per selected route. The JSON report
// contains route and attempt metadata plus derived checks, never source text,
// model output, audio, credentials or the routing file contents.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gemini-hackathon/app/internal/ai"
)

const (
	exitOK     = 0
	exitFailed = 1
	exitUsage  = 2

	maxImageBytes   = 10 << 20
	maxContextBytes = 64 << 10
	redactedID      = "[redacted: echoes content]"
)

var (
	allTasks     = []string{"ocr", "annotation", "speech"}
	languageCode = regexp.MustCompile(`^[A-Za-z-]{1,16}$`)
)

type qualificationReport struct {
	CheckedAt string       `json:"checked_at"`
	Live      bool         `json:"live"`
	Tasks     []taskReport `json:"tasks"`
}

type taskReport struct {
	Task                  string            `json:"task"`
	Profile               string            `json:"profile"`
	Gateway               string            `json:"gateway"`
	Region                string            `json:"region"`
	Model                 string            `json:"model"`
	Voice                 string            `json:"voice,omitempty"`
	Format                string            `json:"format,omitempty"`
	Status                string            `json:"status"`
	ErrorKind             ai.ErrorKind      `json:"error_kind,omitempty"`
	HTTPStatus            int               `json:"http_status,omitempty"`
	Error                 string            `json:"error,omitempty"`
	Attempts              []ai.CallMetadata `json:"attempts,omitempty"`
	Checks                map[string]any    `json:"checks,omitempty"`
	MetadataEchoesContent bool              `json:"metadata_echoes_content,omitempty"`
}

type inputs struct {
	image     []byte
	imageMIME string
	context   string
	selection string
	audioOut  string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, lookupEnv func(string) string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("qualify-routes", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", lookupEnv("AI_ROUTING_CONFIG"), "routing JSON path (default $AI_ROUTING_CONFIG)")
	taskList := flags.String("tasks", strings.Join(allTasks, ","), "comma-separated tasks to qualify")
	live := flags.Bool("live", false, "make one real, possibly billed, call per selected task")
	imagePath := flags.String("image", "", "OCR sample page (PNG, JPEG, WebP or GIF)")
	contextPath := flags.String("context-file", "", "UTF-8 Japanese passage used as annotation context")
	selection := flags.String("selection", "", "text to annotate and speak; must appear in -context-file when given")
	audioOut := flags.String("audio-out", "", "new file for synthesized audio, for decoding and listening")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	usage := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "qualify-routes: "+format+"\n", a...)
		return exitUsage
	}
	if flags.NArg() > 0 {
		return usage("unexpected arguments")
	}
	tasks, err := parseTasks(*taskList)
	if err != nil {
		return usage("%v", err)
	}
	if strings.TrimSpace(*configPath) == "" {
		return usage("-config or AI_ROUTING_CONFIG is required")
	}
	cfg, err := ai.LoadRoutingConfig(*configPath, lookupEnv)
	if err != nil {
		return usage("invalid routing configuration: %v", err)
	}
	report := qualificationReport{CheckedAt: time.Now().UTC().Format(time.RFC3339), Live: *live}
	if !*live {
		for _, task := range tasks {
			result := routeReport(cfg, task)
			result.Status = "validated"
			report.Tasks = append(report.Tasks, result)
		}
		return writeReport(stdout, stderr, report, exitOK)
	}
	in, err := readInputs(tasks, *imagePath, *contextPath, *selection, *audioOut)
	if err != nil {
		return usage("%v", err)
	}

	collector := &observations{byTask: map[string][]ai.CallMetadata{}}
	client, err := ai.NewRoutedClient(cfg, ai.RoutedClientOptions{Observe: collector.observe})
	if err != nil {
		return usage("invalid routing configuration: %v", err)
	}
	code := exitOK
	for _, task := range tasks {
		result := routeReport(cfg, task)
		outputs, callErr := qualify(ctx, client, task, in, &result)
		result.Attempts = collector.take(task)
		if callErr != nil {
			result.Status = "failed"
			describeError(callErr, &result)
		} else {
			result.Status = "passed"
		}
		// Provider IDs may contain Unicode letters, so check them against both
		// what was sent and what came back before they reach the report.
		if redactEchoes(result.Attempts, append([]string{in.context, in.selection}, outputs...)) {
			result.MetadataEchoesContent = true
			result.Status = "failed"
		}
		if result.Status != "passed" {
			code = exitFailed
		}
		report.Tasks = append(report.Tasks, result)
	}
	return writeReport(stdout, stderr, report, code)
}

func parseTasks(raw string) ([]string, error) {
	var tasks []string
	for _, task := range strings.Split(raw, ",") {
		task = strings.TrimSpace(task)
		if !slices.Contains(allTasks, task) {
			return nil, fmt.Errorf("unknown task %q; use ocr, annotation or speech", task)
		}
		if !slices.Contains(tasks, task) {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func readInputs(tasks []string, imagePath, contextPath, selection, audioOut string) (inputs, error) {
	in := inputs{selection: strings.TrimSpace(selection), audioOut: audioOut}
	if slices.Contains(tasks, "ocr") {
		if imagePath == "" {
			return in, errors.New("-image is required for ocr")
		}
		data, err := readBounded(imagePath, maxImageBytes)
		if err != nil {
			return in, fmt.Errorf("cannot read -image: %w", err)
		}
		in.image, in.imageMIME = data, http.DetectContentType(data)
		if !slices.Contains([]string{"image/png", "image/jpeg", "image/webp", "image/gif"}, in.imageMIME) {
			return in, errors.New("-image must be PNG, JPEG, WebP or GIF")
		}
	}
	if slices.Contains(tasks, "annotation") || contextPath != "" {
		if contextPath == "" {
			return in, errors.New("-context-file is required for annotation")
		}
		data, err := readBounded(contextPath, maxContextBytes)
		if err != nil {
			return in, fmt.Errorf("cannot read -context-file: %w", err)
		}
		in.context = strings.TrimSpace(string(data))
	}
	if (slices.Contains(tasks, "annotation") || slices.Contains(tasks, "speech")) && in.selection == "" {
		return in, errors.New("-selection is required for annotation and speech")
	}
	if in.context != "" && !strings.Contains(in.context, in.selection) {
		return in, errors.New("-selection must appear in -context-file")
	}
	if audioOut != "" {
		if !slices.Contains(tasks, "speech") {
			return in, errors.New("-audio-out requires the speech task")
		}
		if _, err := os.Lstat(audioOut); !errors.Is(err, os.ErrNotExist) {
			return in, errors.New("-audio-out must name a new file")
		}
	}
	return in, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("file not readable")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, errors.New("file not readable")
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, fmt.Errorf("file must be 1 to %d bytes", limit)
	}
	return data, nil
}

func routeReport(cfg ai.RoutingConfig, task string) taskReport {
	var route ai.TaskRoute
	switch task {
	case "ocr":
		route = cfg.Tasks.OCR
	case "annotation":
		route = cfg.Tasks.Annotation
	default:
		route = cfg.Tasks.Speech
	}
	profile := cfg.Profiles[route.Profile]
	return taskReport{Task: task, Profile: route.Profile, Gateway: profile.Kind, Region: profile.Region, Model: route.Model, Voice: route.Voice, Format: route.Format}
}

// qualify makes the task's single client call, records derived checks and
// returns model output only for the echo check; output never enters the report.
func qualify(ctx context.Context, client ai.Client, task string, in inputs, result *taskReport) ([]string, error) {
	switch task {
	case "ocr":
		resp, err := client.OCR(ctx, in.image, in.imageMIME)
		if err != nil {
			return nil, err
		}
		result.Checks = map[string]any{
			"text_runes":            len([]rune(resp.RawText)),
			"contains_japanese":     containsJapanese(resp.RawText),
			"structured_json_valid": resp.StructuredJSON == "" || json.Valid([]byte(resp.StructuredJSON)),
		}
		if languageCode.MatchString(resp.Language) {
			result.Checks["language"] = resp.Language
		}
		return []string{resp.RawText, resp.StructuredJSON}, nil
	case "annotation":
		resp, err := client.Annotate(ctx, in.context, in.selection)
		if err != nil {
			return nil, err
		}
		fields := map[string]string{
			"translation": resp.Translation, "contextual_explanation": resp.ContextualExplanation,
			"usage_example": resp.UsageExample, "when_to_use": resp.WhenToUse, "word_breakdown": resp.WordBreakdown,
			"alternative_meanings": resp.AlternativeMeanings, "pronunciation.kana": resp.Pronunciation.Kana,
		}
		present := map[string]bool{}
		all := true
		var outputs []string
		for name, value := range fields {
			present[name] = strings.TrimSpace(value) != ""
			all = all && present[name]
			outputs = append(outputs, value)
		}
		result.Checks = map[string]any{"required_fields_present": all, "fields_present": present, "romaji_present": resp.Pronunciation.Romaji != ""}
		return append(outputs, resp.Pronunciation.Romaji, resp.Meaning, resp.AlternativeMeaning), nil
	default:
		resp, err := client.SynthesizeSpeech(ctx, in.selection, in.context)
		if err != nil {
			return nil, err
		}
		result.Checks = map[string]any{"audio_bytes": len(resp.Audio), "mime_type": resp.MIMEType, "mp3_header": mp3Header(resp.Audio)}
		if in.audioOut != "" {
			if err := writeNew(in.audioOut, resp.Audio); err != nil {
				result.Checks["audio_written"] = false
				return nil, err
			}
			result.Checks["audio_written"] = true
		}
		if !mp3Header(resp.Audio) {
			return nil, errors.New("speech audio lacks an MP3 header")
		}
		return nil, nil
	}
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("cannot create -audio-out file")
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return errors.New("cannot write -audio-out file")
	}
	if err := f.Close(); err != nil {
		return errors.New("cannot write -audio-out file")
	}
	return nil
}

func describeError(err error, result *taskReport) {
	var providerErr *ai.ProviderError
	if errors.As(err, &providerErr) {
		// ProviderError messages exclude upstream bodies, URLs and credentials.
		result.ErrorKind, result.HTTPStatus, result.Error = providerErr.Kind, providerErr.StatusCode, providerErr.Error()
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		result.Error = "cancelled or timed out"
		return
	}
	result.Error = err.Error()
}

type observations struct {
	mu     sync.Mutex
	byTask map[string][]ai.CallMetadata
}

func (o *observations) observe(_ context.Context, meta ai.CallMetadata) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byTask[meta.Task] = append(o.byTask[meta.Task], meta)
}

func (o *observations) take(task string) []ai.CallMetadata {
	o.mu.Lock()
	defer o.mu.Unlock()
	attempts := o.byTask[task]
	delete(o.byTask, task)
	return attempts
}

// redactEchoes replaces provider IDs that contain a fragment of the sample or
// its output. Reported models are already validated against the configuration.
func redactEchoes(attempts []ai.CallMetadata, texts []string) bool {
	fragments := contentFragments(texts)
	echoed := false
	for i := range attempts {
		id := strings.ToLower(attempts[i].RequestID)
		for _, fragment := range fragments {
			if id != "" && strings.Contains(id, fragment) {
				attempts[i].RequestID = redactedID
				echoed = true
				break
			}
		}
	}
	return echoed
}

// contentFragments returns every 3-rune window of Japanese script and every
// ASCII word of at least 8 letters, long enough to avoid matching random IDs.
func contentFragments(texts []string) []string {
	seen := map[string]bool{}
	var fragments []string
	add := func(fragment string) {
		if !seen[fragment] {
			seen[fragment] = true
			fragments = append(fragments, fragment)
		}
	}
	for _, text := range texts {
		var japanese []rune
		flush := func() {
			for i := 0; i+3 <= len(japanese); i++ {
				add(string(japanese[i : i+3]))
			}
			japanese = japanese[:0]
		}
		for _, r := range text {
			if isJapanese(r) {
				japanese = append(japanese, r)
				continue
			}
			flush()
		}
		flush()
		for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return r > unicode.MaxASCII || !unicode.IsLetter(r) }) {
			if len(word) >= 8 {
				add(word)
			}
		}
	}
	return fragments
}

func isJapanese(r rune) bool {
	return unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han)
}

func containsJapanese(text string) bool {
	return strings.ContainsFunc(text, isJapanese)
}

func mp3Header(audio []byte) bool {
	return len(audio) >= 3 && (string(audio[:3]) == "ID3" || (audio[0] == 0xff && audio[1]&0xe0 == 0xe0))
}

func writeReport(stdout, stderr io.Writer, report qualificationReport, code int) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(stderr, "qualify-routes: cannot write report")
		return exitFailed
	}
	return code
}
