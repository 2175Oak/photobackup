// Package gemini เรียก Gemini API (REST) เพื่อให้ AI อธิบายภาพและตั้งแท็กให้
// ใช้เฉพาะ standard library จึงไม่ต้องเพิ่ม dependency และทดสอบได้ด้วย httptest
package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultModel   = "gemini-3.8-flash"
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	maxTags        = 10
	maxRetries     = 3
)

// ErrNoKey ใช้เมื่อยังไม่ได้ตั้ง GEMINI_API_KEY
var ErrNoKey = errors.New("ยังไม่ได้ตั้งค่า GEMINI_API_KEY ในไฟล์ .env")

// APIError คือ error ที่ Gemini ตอบกลับมา (Status = HTTP status code)
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Gemini ตอบกลับ %d: %s", e.Status, e.Message)
}

// Fatal = ลองรูปถัดไปก็ไม่น่าจะผ่าน (key ผิด/ไม่มีสิทธิ์/ชื่อโมเดลผิด) ควรหยุดทั้งงาน
func (e *APIError) Fatal() bool {
	return e.Status == http.StatusBadRequest || e.Status == http.StatusUnauthorized ||
		e.Status == http.StatusForbidden || e.Status == http.StatusNotFound
}

// Result คือสิ่งที่ AI วิเคราะห์ได้จากภาพหนึ่งภาพ
type Result struct {
	Caption string   `json:"caption"`
	Tags    []string `json:"tags"`
}

type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
	backoff time.Duration
}

// New สร้าง client ถ้า model ว่างจะใช้ DefaultModel
func New(apiKey, model string) *Client {
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	return &Client{
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 90 * time.Second},
		backoff: 2 * time.Second,
	}
}

func (c *Client) Configured() bool { return c != nil && c.apiKey != "" }
func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

const prompt = `วิเคราะห์ภาพนี้แล้วตอบเป็น JSON เท่านั้น
- caption: คำอธิบายภาพเป็นภาษาไทย 1-2 ประโยค บอกว่าในภาพมีอะไร ใคร/สิ่งใด กำลังทำอะไร อยู่ที่ไหน บรรยากาศเป็นอย่างไร
- tags: แท็กภาษาไทย 5-8 คำ สั้นๆ เป็นคำนามหรือวลีสั้น (เช่น ทะเล, พระอาทิตย์ตก, อาหาร, คน, สุนัข) ห้ามใส่เครื่องหมาย # และห้ามซ้ำกัน`

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}
type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

// Describe ส่งภาพ (JPEG/PNG/WebP/HEIC ...) ให้ Gemini แล้วคืนคำอธิบายกับแท็ก
// ลองซ้ำเองเมื่อติด rate limit (429) หรือเซิร์ฟเวอร์ไม่ว่าง (5xx)
func (c *Client) Describe(ctx context.Context, img []byte, mime string) (Result, error) {
	if !c.Configured() {
		return Result{}, ErrNoKey
	}

	body, err := json.Marshal(map[string]any{
		"contents": []any{map[string]any{
			"parts": []part{
				{Text: prompt},
				{InlineData: &inlineData{MimeType: mime, Data: base64.StdEncoding.EncodeToString(img)}},
			},
		}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"responseSchema": map[string]any{
				"type": "OBJECT",
				"properties": map[string]any{
					"caption": map[string]any{"type": "STRING"},
					"tags":    map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
				},
				"required": []string{"caption", "tags"},
			},
		},
	})
	if err != nil {
		return Result{}, err
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(c.backoff * time.Duration(1<<(attempt-1))):
			}
		}
		res, retry, err := c.once(ctx, body)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return Result{}, lastErr
}

func (c *Client) once(ctx context.Context, body []byte) (res Result, retry bool, err error) {
	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		// ต่อเน็ตไม่ได้/หมดเวลา: ลองใหม่ได้ แต่ถ้าผู้ใช้กดยกเลิกก็หยุดเลย
		return Result{}, ctx.Err() == nil, fmt.Errorf("เรียก Gemini ไม่สำเร็จ: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Result{}, true, err
	}

	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return Result{}, retryable, &APIError{Status: resp.StatusCode, Message: msg}
	}

	res, err = parseResponse(raw)
	// คำตอบที่อ่านไม่ได้ (เช่น ถูกตัดกลางคัน) ลองถามใหม่ได้
	return res, err != nil, err
}

func parseResponse(raw []byte) (Result, error) {
	var r struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, fmt.Errorf("อ่านคำตอบของ Gemini ไม่ได้: %w", err)
	}
	if len(r.Candidates) == 0 {
		if r.PromptFeedback.BlockReason != "" {
			return Result{}, fmt.Errorf("Gemini ปฏิเสธภาพนี้ (%s)", r.PromptFeedback.BlockReason)
		}
		return Result{}, errors.New("Gemini ไม่ส่งคำตอบกลับมา")
	}
	var sb strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return Result{}, fmt.Errorf("Gemini ตอบว่าง (%s)", r.Candidates[0].FinishReason)
	}

	var out Result
	if err := json.Unmarshal([]byte(stripFence(text)), &out); err != nil {
		return Result{}, fmt.Errorf("รูปแบบคำตอบของ Gemini ไม่ถูกต้อง: %w", err)
	}
	out.Caption = strings.TrimSpace(out.Caption)
	out.Tags = CleanTags(out.Tags)
	if out.Caption == "" {
		return Result{}, errors.New("Gemini ไม่ได้ให้คำอธิบายภาพ")
	}
	return out, nil
}

// stripFence ตัด ```json ... ``` ออก เผื่อโมเดลห่อคำตอบมา
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

// CleanTags ตัดช่องว่างและ # ทำเป็นตัวพิมพ์เล็ก ตัดซ้ำ และจำกัดจำนวน
// (ใช้ตอนบันทึกด้วย เพื่อให้ "Dog" กับ "dog" เป็นแท็กเดียวกัน)
func CleanTags(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#")))
		if t == "" || len([]rune(t)) > 40 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == maxTags {
			break
		}
	}
	return out
}
