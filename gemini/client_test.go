package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The per-photo caption action is still used for older tag-only rows, even
// though the former single-image analysis queue and Live transport are gone.
func TestDescribeStillSupportsLegacyCaptionAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/"+DefaultTagModel+":generateContent" || r.Header.Get("x-goog-api-key") != "test" {
			t.Error("incorrect REST model/auth")
		}
		var payload struct{ Contents []struct{ Parts []part } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if len(payload.Contents) != 1 || len(payload.Contents[0].Parts) != 2 {
			t.Error("missing image or prompt")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": `{"caption":"สุนัขนอนบนโซฟา","tags":["สุนัข","โซฟา"]}`}}}}}})
	}))
	defer server.Close()
	client := New("test", "")
	client.baseURL = server.URL
	result, err := client.Describe(context.Background(), []byte("image"), "image/jpeg")
	if err != nil || result.Caption != "สุนัขนอนบนโซฟา" || len(result.Tags) != 2 {
		t.Fatalf("retained action failed: %+v %v", result, err)
	}
}
