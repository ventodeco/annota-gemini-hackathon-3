package ai

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net/http"
)

type speechTask struct{ gateway *taskGateway }

func (s *speechTask) SynthesizeSpeech(ctx context.Context, text, contextText string) (*SpeechResponse, error) {
	g := s.gateway
	if text == "" {
		return nil, g.failure(ErrorKindConfig, 0, "speech requires selected text", nil)
	}
	// Context is intentionally not spoken. The model/voice determines pronunciation.
	var audio []byte
	path := "/audio/speech"
	limit := int64(32 << 20)
	body := map[string]any{"model": g.route.Model, "input": text, "voice": g.route.Voice, "response_format": g.route.Format}
	if g.profile.Kind == "modelverse" {
		path = "/t2a_v2"
		limit = 64 << 20
		body = map[string]any{"model": g.route.Model, "text": text, "stream": false, "output_format": "hex", "language_boost": "Japanese", "voice_setting": map[string]any{"voice_id": g.route.Voice, "speed": 1, "vol": 1, "pitch": 0}, "audio_setting": map[string]any{"sample_rate": 32000, "bitrate": 128000, "format": g.route.Format, "channel": 1}}
	}
	err := g.call(ctx, path, body, limit, func(data []byte, header http.Header, meta *CallMetadata) error {
		if g.profile.Kind == "modelverse" {
			var response struct {
				Data *struct {
					Audio  string `json:"audio"`
					Status int    `json:"status"`
				} `json:"data"`
				BaseResp *struct {
					StatusCode *int   `json:"status_code"`
					StatusMsg  string `json:"status_msg"`
				} `json:"base_resp"`
				ExtraInfo struct {
					AudioFormat     string `json:"audio_format"`
					UsageCharacters *int64 `json:"usage_characters"`
				} `json:"extra_info"`
				TraceID string `json:"trace_id"`
			}
			if json.Unmarshal(data, &response) != nil || response.BaseResp == nil || response.BaseResp.StatusCode == nil {
				return g.failure(ErrorKindUpstream, meta.StatusCode, "malformed speech response", nil)
			}
			meta.Characters = response.ExtraInfo.UsageCharacters
			if meta.RequestID == "" {
				meta.RequestID = g.safeID(response.TraceID)
			}
			if code := *response.BaseResp.StatusCode; code != 0 {
				return g.failure(classifyMiniMaxStatus(code, response.BaseResp.StatusMsg), meta.StatusCode, "speech gateway reported a business error", nil)
			}
			if response.Data == nil || response.Data.Status != 2 || response.ExtraInfo.AudioFormat != "mp3" {
				return g.failure(ErrorKindUpstream, meta.StatusCode, "speech audio is incomplete or has an unsupported format", nil)
			}
			decoded, err := hex.DecodeString(response.Data.Audio)
			if err != nil || len(decoded) > 32<<20 {
				return g.failure(ErrorKindUpstream, meta.StatusCode, "invalid speech audio encoding or size", nil)
			}
			audio = decoded
		} else {
			contentType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
			if err != nil || (contentType != "audio/mpeg" && contentType != "audio/mp3") {
				return g.failure(ErrorKindUpstream, meta.StatusCode, "speech response is not MP3 audio", nil)
			}
			audio = data
		}
		if !plausibleMP3(audio) {
			return g.failure(ErrorKindUpstream, meta.StatusCode, "speech response contains no valid MP3 header", nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &SpeechResponse{Audio: audio, MIMEType: "audio/mpeg"}, nil
}

// plausibleMP3 checks a complete first MPEG Layer III frame, not just an ID3 tag.
// It is a framing check; live qualification must still decode and listen to audio.
func plausibleMP3(audio []byte) bool {
	if len(audio) >= 10 && string(audio[:3]) == "ID3" {
		size := 0
		for _, b := range audio[6:10] {
			if b&0x80 != 0 {
				return false
			}
			size = (size << 7) | int(b)
		}
		offset := 10 + size
		if audio[3] == 4 && audio[5]&0x10 != 0 {
			offset += 10
		}
		if offset > len(audio) {
			return false
		}
		audio = audio[offset:]
	}
	if len(audio) < 4 || audio[0] != 0xff || audio[1]&0xe0 != 0xe0 || audio[1]&0x06 != 0x02 {
		return false
	}
	version := (audio[1] >> 3) & 3
	bitrateIndex := audio[2] >> 4
	sampleIndex := (audio[2] >> 2) & 3
	if version == 1 || bitrateIndex == 0 || bitrateIndex == 15 || sampleIndex == 3 {
		return false
	}
	rates := [3]int{44100, 48000, 32000}
	rate := rates[sampleIndex]
	bitrates := [15]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	multiplier := 144000
	if version != 3 {
		bitrates = [15]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
		multiplier = 72000
		rate /= 2
		if version == 0 {
			rate /= 2
		}
	}
	frameSize := multiplier*bitrates[bitrateIndex]/rate + int((audio[2]>>1)&1)
	return len(audio) >= frameSize
}
