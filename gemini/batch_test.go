package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/joho/godotenv"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var sampleTags = []string{"ทะเล", "น้ำ", "ท้องฟ้า", "กลางแจ้ง", "ธรรมชาติ"}

func TestBatchValidatesIDsAndKeepsPartialResults(t *testing.T) {
	raw := `{"items":[{"id":1,"caption":"ภาพทดสอบ","tags":["ทะเล","น้ำ","ท้องฟ้า","กลางแจ้ง","ธรรมชาติ"]},{"id":2,"caption":"ภาพทดสอบ","tags":["a","b","c","d","e"]},{"id":2,"caption":"ภาพทดสอบ","tags":["f","g","h","i","j"]},{"id":999,"caption":"ภาพทดสอบ","tags":["a","b","c","d","e"]}]}`
	result, err := parseTagItems([]byte(raw), map[int64]bool{1: true, 2: true, 3: true})
	if err == nil || len(result) != 1 || len(result[1].Tags) != 5 {
		t.Fatalf("unsafe partial response: %v %v", result, err)
	}
}

func TestBatchHTTPAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test" {
			t.Error("missing key")
		}
		var payload struct {
			Contents         []struct{ Parts []part }
			GenerationConfig map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.Contents) != 1 || len(payload.Contents[0].Parts) != 5 {
			t.Error("images not separately labelled")
		}
		if payload.GenerationConfig["responseMimeType"] != "application/json" {
			t.Error("missing schema mode")
		}
		data, _ := json.Marshal(map[string]any{"items": []any{map[string]any{"id": 2, "caption": "ภาพทดสอบ", "tags": sampleTags}, map[string]any{"id": 1, "caption": "ภาพทดสอบ", "tags": sampleTags}}})
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(data)}}}}}, "usageMetadata": map[string]any{"promptTokenCount": 300, "candidatesTokenCount": 100, "thoughtsTokenCount": 10}})
	}))
	defer server.Close()
	b := NewBatch("test", DefaultTagModel, BatchConfig{Size: 8, Workers: 4, RPM: 10000, TPM: 10000000, MaxRequests: 10})
	b.client.baseURL = server.URL
	result, err := b.Tag(context.Background(), []TagImage{{ID: 1, Data: []byte("a"), MIME: "image/jpeg"}, {ID: 2, Data: []byte("b"), MIME: "image/jpeg"}})
	if err != nil || len(result) != 2 {
		t.Fatalf("%v %v", result, err)
	}
	stats := b.Stats()
	if stats.InputTokens != 300 || stats.OutputTokens != 110 || stats.Requests != 1 || stats.Active != 0 || !stats.TokensReported {
		t.Fatalf("%+v", stats)
	}
}

func TestBatchRequiresShortCaptionForEachID(t *testing.T) {
	items := []any{
		map[string]any{"id": 1, "caption": "  สุนัขนอนบนโซฟา  ", "tags": sampleTags},
		map[string]any{"id": 2, "tags": sampleTags},
		map[string]any{"id": 3, "caption": strings.Repeat("ก", 121), "tags": sampleTags},
	}
	raw, _ := json.Marshal(map[string]any{"items": items})
	results, err := parseTagItems(raw, map[int64]bool{1: true, 2: true, 3: true})
	if err == nil || len(results) != 1 || results[1].Caption != "สุนัขนอนบนโซฟา" {
		t.Fatalf("invalid caption accepted or valid result lost: %+v %v", results, err)
	}
}

func TestBatchTwelveImagesKeepsIDsAndRejectsThirteen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Contents         []struct{ Parts []part }
			GenerationConfig map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if len(payload.Contents) != 1 || len(payload.Contents[0].Parts) != 25 {
			t.Error("expected 12 labelled images")
		}
		if payload.GenerationConfig["maxOutputTokens"].(float64) < 4608 {
			t.Error("insufficient output budget")
		}
		items := []any{}
		for id := 12; id >= 1; id-- {
			items = append(items, map[string]any{"id": id, "caption": "ภาพ " + strconv.Itoa(id), "tags": sampleTags})
		}
		data, _ := json.Marshal(map[string]any{"items": items})
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(data)}}}}}})
	}))
	defer server.Close()
	b := NewBatch("test", DefaultTagModel, BatchConfig{Size: 12, Workers: 4, RPM: 14, TPM: 200000, MaxRequests: 2})
	b.client.baseURL = server.URL
	images := []TagImage{}
	for i := 1; i <= 12; i++ {
		images = append(images, TagImage{ID: int64(i), Data: []byte("image"), MIME: "image/jpeg"})
	}
	results, err := b.Tag(context.Background(), images)
	if err != nil || len(results) != 12 || b.Settings().Size != 12 {
		t.Fatalf("%+v %v", results, err)
	}
	for id, result := range results {
		if result.Caption != "ภาพ "+strconv.Itoa(int(id)) {
			t.Fatal("caption assigned by order instead of ID")
		}
	}
	_, err = b.Tag(context.Background(), append(images, TagImage{ID: 13}))
	if err == nil || b.Stats().Requests != 1 {
		t.Fatal("oversized batch reached server")
	}
}

func TestBatchAdmissionCancellationAndCooldown(t *testing.T) {
	b := NewBatch("test", DefaultTagModel, BatchConfig{Size: 8, Workers: 4, RPM: 14, TPM: 200000, MaxRequests: 2})
	b.next = time.Now().Add(time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := b.acquire(ctx, 8); err == nil {
		t.Fatal("did not cancel queue wait")
	}
	if b.Stats().Requests != 0 {
		t.Fatal("counted waiting as request")
	}
	b.stats.Active = 1
	b.stats.Concurrency = 4
	b.finish(8, 0, 0, false, 429, time.Minute, time.Second, false)
	if b.Stats().Concurrency != 2 || time.Until(b.cooldown) < 59*time.Second {
		t.Fatal("quota did not back off")
	}
	b.ResetStats()
	if time.Until(b.cooldown) < 59*time.Second {
		t.Fatal("new run erased quota cooldown")
	}
}

// Opt-in only: two synthetic images, no private photo or SQLite access.
func TestBatchRealAPISmoke(t *testing.T) {
	if os.Getenv("PHOTO_TEST_REAL_API") != "1" {
		t.Skip("explicit opt-in required")
	}
	env, err := godotenv.Read("../.env")
	if err != nil {
		t.Fatal("cannot load test credentials")
	}
	b := NewBatch(env["GEMINI_API_KEY"], DefaultTagModel, BatchConfig{Size: 8, Workers: 1, RPM: 14, TPM: 200000, MaxRequests: 1})
	images := []TagImage{}
	count := 2
	if n, err := strconv.Atoi(os.Getenv("PHOTO_TEST_REAL_IMAGES")); err == nil {
		count = max(1, min(MaxBatchSize, n))
	}
	for i := 1; i <= count; i++ {
		im := image.NewRGBA(image.Rect(0, 0, 256, 256))
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				im.Set(x, y, color.RGBA{uint8(i * 90), uint8(x), uint8(y), 255})
			}
		}
		var buf bytes.Buffer
		_ = jpeg.Encode(&buf, im, nil)
		images = append(images, TagImage{ID: int64(i), Data: buf.Bytes(), MIME: "image/jpeg"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	results, err := b.Tag(ctx, images)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != count {
		t.Fatal("missing synthetic results")
	}
	t.Logf("synthetic smoke: model=%s valid_images=%d stats=%+v", b.Model(), len(results), b.Stats())
}

func TestBatchErrorDoesNotEchoResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte("secret-test-key"))
	}))
	defer server.Close()
	b := NewBatch("test", DefaultTagModel, BatchConfig{Size: 1, Workers: 1, RPM: 14, TPM: 200000, MaxRequests: 1})
	b.client.baseURL = server.URL
	_, err := b.Tag(context.Background(), []TagImage{{ID: 1, Data: []byte("a"), MIME: "image/jpeg"}})
	if err == nil || strings.Contains(err.Error(), "secret-test-key") {
		t.Fatalf("unsafe error: %v", err)
	}
}
