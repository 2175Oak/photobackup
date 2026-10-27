# Tagging pipeline

The default path is now stateless, multi-image `generateContent` with
`gemini-3.5-flash-lite`. RPM/TPM in configuration are local pacing budgets, not discovered
account limits. A successful model lookup does not establish a quota allowance.

## Flow

1. Acquire the service operation lock; recover interrupted jobs in SQLite.
2. Select pending photos (at most 100 for the sample button). Legacy captions
   and completed tag-only rows both count as completed and are skipped.
3. Stream SHA-256 over originals; group only byte-identical photos. Keep file
   metadata in memory, not all image buffers. Hashing can be cancelled.
4. Workers load at most one batch each. Reuse tags with the same content hash
   and model/prompt version. Otherwise prepare 768px JPEG thumbnails, cached
   atomically beside the configured database in `analysis-cache/`.
5. Send up to twelve separately labelled images. Large HEIC/WebP inputs split
   into smaller requests; individual images over 12 MiB require conversion.
6. Start with one active request; after three successful batches increase the
   permitted concurrency by one, to the configured maximum. Pace starts by
   both RPM and estimated input TPM. Reconcile the reservation with actual
   reported input tokens. Other apps using the project are not accounted for.
7. Each HTTP attempt gets 90 seconds **after** admission. Queue/cooldown time
   does not consume this deadline. 429/5xx reduce concurrency and impose a
   shared cooldown; respect `Retry-After` when supplied. Stop is cancellation-aware.
8. Validate returned IDs, 5–8 cleaned tags, and a nonempty caption of at most 120 Unicode code points. The prompt requests one short Thai sentence. Missing or oversized captions make only that ID retryable. Duplicate IDs are rejected;
   unknown IDs are ignored. Save valid items immediately. Retry only missing
   or invalid items, at most three attempts. Never infer identity by order.
9. Persist caption, tags, done state, content hash and version transactionally, with
   serialized writes. Retry/error states and attempt counts remain in SQLite.
   Fatal auth/model errors or exhausted quota retries stop dispatch. Drain
   completed work so successful results survive Stop and partial failures.
10. Search matches both tags and captions. New batches return both in the same request. A separate per-photo action generates
    a caption for legacy tag-only rows on demand, preserving existing tags. It uses the tag model
    with a single-image REST request and may consume additional quota.

## Settings

See `.env.example`. Defaults: twelve images/request, up to four concurrent
requests, 14 RPM, 200,000 estimated input TPM, maximum 200 requests per run.
The request cap is a local safety budget, not RPD accounting. Check the actual
Flash-Lite quota in AI Studio. Increasing concurrency cannot overcome a quota.
There is no automatic model fallback or billing-tier change.

`ANALYSIS_BATCH_SIZE=8` can be compared with `12` using the sample button.
Each sample processes up to 100 **pending** photos and keeps its real results;
the subsequent full run skips those photos. For a controlled quality comparison,
use a separate test database and the same representative images. The app does
not erase completed analyses to rerun a benchmark.

## Measurement and limits

The UI reports completed photos/minute, request count, cached/duplicate count,
actual reported input/output-plus-thinking tokens, active calls and wait time.
`logs/tag-runs.jsonl` keeps per-run counters, wall time and accumulated API time,
without photos, paths, keys, tags or raw API errors. Missing token usage is not
interpreted as zero. Failed calls can be billed even when usage is not reported;
the displayed numbers are not a billing statement.

Confirmed during development: two synthetic images in one real API request
returned valid per-ID tags in about 6.2 seconds, reporting 586 input and 143 output
tokens. This is a protocol smoke test, not a speed or tag-quality benchmark for
the user's collection. No 1,000-photo/10-minute guarantee is made.

At twelve images/request, 1,000 unique images require at least 84 calls. Reaching
ten minutes requires roughly 100 successful photos/minute plus enough RPM, TPM
and daily quota. Combining images reduces call/prompt overhead; it does not
eliminate image tokens. Low media resolution can miss small details and text.

The thumbnail cache is persistent and can be removed while the app is stopped
(it will be recreated). Tag cache entries remain in SQLite and disappear when
their photo rows are deleted. The hash assumes the backed-up source files are
not modified concurrently during a run. Only exact duplicates share tags.

## Verification

`go test ./gemini ./service ./repository ./fileops`

Tests cover partial responses, duplicate/unknown IDs, usage counters, quota
cooldown and cancellation, durable retries, exact duplicate reuse, preservation
of legacy captions, tag-only search, fatal errors and resumption. `node --test frontend/tests/*.test.mjs`
checks progress/error preservation. The real API smoke test is opt-in only via
`PHOTO_TEST_REAL_API=1` and sends two synthetic gradients, never private photos.

Current cache version: tags-caption-v2 plus model ID. Tag-only cache entries are not reused for new combined jobs. Completed legacy rows are not automatically requeued. The two-image timing above was measured on the previous tags-only protocol; it is not a combined-caption performance estimate.

Search now ranks exact matches before spelling-near matches across captions and tags. It handles insertion, deletion, substitution and adjacent transposition over Unicode code points. Queries of 1–2 characters remain exact; 3–7 characters allow one edit and longer words allow two. All query words must match somewhere; selected tag filters and destination remain exact. No external AI calls or semantic synonym expansion are used. A local 1,000-photo ranking benchmark took about 10 ms (excluding SQLite fetch). A synthetic 12-image API request returned all captions/tags in about 5.1 seconds; this does not predict real collection speed.
