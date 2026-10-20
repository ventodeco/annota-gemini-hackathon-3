# Issue 34 gateway qualification evidence

**Status: pending.** No route has been qualified with a real provider. As of 2026-10-20 there were no private gateway credentials and no approved region/model/voice choices (#35). Local contract tests do not count as qualification.

## Procedure

Run from `backend/` with the operator's routing file and only the active profiles' keys in the process environment. Validate first; this mode makes no network calls:

```bash
rtk proxy go run ./cmd/qualify-routes -config /absolute/path/routing.json
```

Then make one bounded call per route. Every live run can be billed, so rerun only to investigate a concrete failure:

```bash
rtk proxy go run ./cmd/qualify-routes -config /absolute/path/routing.json -live \
  -image /path/to/lawful-japanese-page.png \
  -context-file /path/to/japanese-passage.txt \
  -selection '<phrase inside the passage>' \
  -audio-out /tmp/annota-qualification-speech.mp3
```

`-tasks ocr,annotation,speech` limits the run to a subset. Each task makes one client call. Retries come only from the profile's `max_attempts`, after 429/503 within its deadline.

The JSON report contains:

- the selected profile, gateway, region, model and voice/format
- each attempt's provider request ID, reported model, HTTP status, latency, token/character usage and cost (`null` means the gateway did not report it)
- a sanitized error kind
- derived checks: OCR rune count, Japanese-script presence, structured-JSON validity and whether the reported language is Japanese (the language value itself is not copied); annotation required-field presence; speech byte count, MIME and MP3 header

It never contains the sample text, OCR or annotation output, audio, keys or the routing file. A provider ID is replaced with `[redacted: echoes content]` and fails the run (`metadata_echoes_content`) if it is not an opaque ASCII token (letters, digits, `.`, `_`, `:`, `-`; at most 128 characters, so no URLs, paths or Japanese script), or if it contains a fragment of the sample or output (any three-character Japanese run or five-letter ASCII word) or a complete short sample/output text. This is a conservative heuristic: an unexpected ID shape fails the run so the operator can review it. Invalid flag values are not echoed in error messages. That answers the request-ID privacy question from the #51 review for the routes actually used. The exit status is nonzero if any task fails.

After a speech run, decode the audio (for example `afconvert /tmp/annota-qualification-speech.mp3 /tmp/annota-qualification-speech.wav -f WAVE -d LEI16` on macOS) and listen for Japanese pronunciation. Delete the sample audio afterwards. Do not commit samples, outputs or audio.

Judging transcription accuracy and annotation quality across a fixed Japanese dataset is #35's benchmark. This record only proves that each route works end to end.

For that scoring, `-live -output-dir <new directory>` also writes the model output to a private local directory: `ocr.txt` (OCR text) and `annotation.json` (the annotation response). Audio still uses `-audio-out`. Note what each location holds:

- **The report stays content-free.** It records only whether each file was written (`output_written`).
- **The directory holds private source and model text.** It must be a new path inside an existing parent. The command creates it with mode 0700 and its files with mode 0600. It never reuses, merges into, or follows a symlink at an existing path. Files are written through a handle to the directory created for the run, so replacing the path mid-run cannot redirect them.
- **Where to put it:** outside the repository, for example under `/tmp`. Never commit it.

An invalid or existing path exits with status 2 before any request. A failed write after a call marks that task failed with a fixed message, and the report is still printed. Delete the directory after scoring.

## Evidence

Fill one row per configured route from the report. Keep unreported usage/cost as `unknown`.

| Date | Task | Gateway / region | Model (voice) | Request ID | Latency | Usage / cost | Checks | Echo-free | Outcome |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| pending | ocr | pending | pending | | | | | | pending |
| pending | annotation | pending | pending | | | | | | pending |
| pending | speech | pending | pending | | | | decode: / listen: | | pending |

Unsupported or failing routes are recorded as blocked with their error kind. They are not replaced automatically.
