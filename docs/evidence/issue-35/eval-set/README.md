# Issue #35 Japanese evaluation set

A small, fixed set for comparing OCR, annotation, and speech routes. All text is original and was written for this repository, so it carries no copyright or personal data. The targets below were fixed on 2026-10-20, before any paid call. **No live results exist yet.**

## Contents

| ID | Image | Ground truth | Covers |
| --- | --- | --- | --- |
| ocr-01 | `images/01-horizontal.png` | `ground-truth/ocr-01-horizontal.txt` | clean horizontal text |
| ocr-02 | `images/02-vertical.png` | `ground-truth/ocr-02-vertical.txt` | vertical (`vertical-rl`) reading order |
| ocr-03 | `images/03-furigana.png` | `ground-truth/ocr-03-furigana.txt` | furigana; truth is base text only |
| ocr-04 | `images/04-mixed-scripts.png` | `ground-truth/ocr-04-mixed-scripts.txt` | kanji/kana/katakana/Latin/digits, full-width punctuation, `¥` |
| ocr-05 | `images/05-dense.png` | `ground-truth/ocr-05-dense.txt` | dense multi-paragraph page, small type |
| ocr-06 | `images/06-blank.png` | `ground-truth/ocr-06-blank.txt` (empty) | blank page (detects hallucination) |
| photo-* | pending | pending | blurry and skewed phone photos. These need user-approved real captures of a printed sample and are not simulated here. |

Furigana readings for ocr-03 are 雨上(あめあ), 公園(こうえん), 子供(こども), 水(みず), 飛(と), 越(こ) and 遊(あそ). A transcript may omit the ruby or keep it in a separate, clearly marked form. Ruby that is interleaved mid-word counts as character errors.

`samples/*.html` are the sources. `render.sh` rebuilds the PNGs with headless Chrome (900×1200, scale 1, macOS Hiragino Mincho). The committed PNGs are the canonical fixtures, because re-rendering with other fonts changes pixels.

### Annotation selections

The context passage is the named ground-truth file. The expected reading is used to check `pronunciation.kana`.

| ID | Context file | Selection | Expected kana |
| --- | --- | --- | --- |
| ann-1 | `ocr-01-horizontal.txt` | 図書館 | としょかん |
| ann-2 | `ocr-02-vertical.txt` | 台所 | だいどころ |
| ann-3 | `ocr-05-dense.txt` | 音読み | おんよみ |
| ann-4 | `ocr-05-dense.txt` | 身につく | みにつく |
| ann-5 | `ocr-04-mixed-scripts.txt` | 新宿駅 | しんじゅくえき |

### Speech utterances

The spoken text is the whole file, used as both `-context-file` and `-selection`.

| ID | File | Expected reading | Kind |
| --- | --- | --- | --- |
| sp-1 | `speech-01.txt` | おはようございます | unambiguous |
| sp-2 | `speech-02.txt` | としょかんでほんをかります | unambiguous |
| sp-3 | `speech-03.txt` | しんじゅくえきのみなみぐちでまっています | unambiguous |
| sp-4 | `speech-04.txt` | きょう (or こんにち) | ambiguous; record only |
| sp-5 | `speech-05-long.txt` | full passage; check 先生/生まれる/生ビール readings | long text, truncation and stability |

## Acceptance targets

A route qualifies for the beta only if all of these hold on this set:

- **OCR:**
  - character error rate (`score_ocr.py`: NFKC, whitespace removed, Levenshtein ÷ truth length) ≤ 0.02 on ocr-01, and ≤ 0.05 on ocr-02, ocr-03 (base text), ocr-04 and ocr-05;
  - ocr-06 returns empty text or an explicit error;
  - no truncation (`finish_reason`/validation error) on any sample;
  - photo samples: recorded when available, not yet gating.
- **Annotation:** 5/5 pass the application's required-field validation, 5/5 `pronunciation.kana` match the expected kana, with no refusal or truncation.
- **Speech:** 5/5 decode, and sp-1 to sp-3 are heard with the expected reading. sp-4 is recorded as an observation. sp-5 is complete, with no clipped ending.
- **Latency (p50 on the beta region):** OCR ≤ 15 s, annotation ≤ 10 s, speech ≤ 5 s (sp-5 excluded).
- **Billed cost per call (provider-reported, not list price):** OCR ≤ US$0.005, annotation ≤ US$0.002, short utterance ≤ US$0.003. Record a missing report as `unknown`, never as zero.

Missing a target disqualifies the route as configured; it does not trigger a retry loop. Change a target only through a dated issue comment made before the run it applies to.

## Procedure (after keys are provided privately)

From `backend/`, put only the active profiles' keys in the process environment. Never write them to files, issues, or logs. Use `../docs/evidence/issue-35/candidate-routing.json`, or a private copy with `tasks.*` switched to the candidate under test (every alternative is listed in the matrix and checked by `TestCandidateRoutingAlternativesAreValid`).

1. Validate the routing file. This makes no network calls:

   ```bash
   rtk proxy go run ./cmd/qualify-routes -config "$PWD/../docs/evidence/issue-35/candidate-routing.json"
   ```

2. Run one live pass per sample and candidate, with `max_attempts` 1. For example, ocr-01 plus ann-1 plus sp-2 in one invocation:

   ```bash
   E=../docs/evidence/issue-35/eval-set
   rtk proxy go run ./cmd/qualify-routes -config "$PWD/$E/../candidate-routing.json" -live \
     -tasks ocr,annotation -image "$E/images/01-horizontal.png" \
     -context-file "$E/ground-truth/ocr-01-horizontal.txt" -selection '図書館'
   rtk proxy go run ./cmd/qualify-routes -config "$PWD/$E/../candidate-routing.json" -live \
     -tasks speech -context-file "$E/ground-truth/speech-02.txt" \
     -selection '図書館で本を借ります。' -audio-out /tmp/annota-35-sp-2.mp3
   ```

   The JSON report gives route correctness, latency, usage/cost and the provider-ID privacy check. Copy the redacted report fields into the matrix results table, and share them with #34 so its route evidence table needs no repeat paid call.

3. Accuracy scoring needs the model output, which the report deliberately omits. Until an opt-in, local-only output option is added to `qualify-routes` as a reviewed follow-up (agreed with #34), capture output through the routed app on a local machine. Save OCR text to a temporary file outside the repo, then run:

   ```bash
   python3 -I score_ocr.py ground-truth/ocr-01-horizontal.txt /tmp/annota-35-ocr-01.txt
   ```

   For annotations, compare `pronunciation.kana` by hand. Listen to the audio, record the observed reading, then delete the audio and output files. Do not commit outputs or audio.

4. Rerun an unchanged sample only to answer a specific question, such as a prompt or route change.
