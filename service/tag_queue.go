package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"photo-backup/gemini"
	"photo-backup/model"
)

type AnalysisMetrics struct {
	gemini.BatchStats
	Phase           string  `json:"phase"`
	Prepared        int     `json:"prepared"`
	Cached          int     `json:"cached"`
	ElapsedSeconds  float64 `json:"elapsedSeconds"`
	PhotosPerMinute float64 `json:"photosPerMinute"`
}
type tagGroup struct {
	hash   string
	photos []model.Photo
}
type tagOutcome struct {
	photo  model.Photo
	err    error
	cached bool
}

func analysisEnvInt(key string, fallback, low, high int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return max(low, min(high, n))
}

// Hash originals, not thumbnails: visually similar files must not inherit tags
// accidentally. Hashing is streamed and cancellation is checked between reads.
func analysisHash(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 128<<10)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func tagCachePath(hash string) string {
	dbPath := os.Getenv("DATABASE")
	if dbPath == "" {
		dbPath = filepath.Join("database", "photo.db")
	}
	return filepath.Join(filepath.Dir(dbPath), "analysis-cache", hash+"-768-v1.jpg")
}

func tagImage(path, hash string) ([]byte, string, error) {
	cache := tagCachePath(hash)
	if data, err := os.ReadFile(cache); err == nil && len(data) > 0 {
		return data, "image/jpeg", nil
	}
	data, mime, err := analysisImage(path)
	if err != nil {
		return nil, "", err
	}
	if mime == "image/jpeg" && os.MkdirAll(filepath.Dir(cache), 0700) == nil {
		// Atomic publication; disk cache failure must not discard an analyzable image.
		if f, e := os.CreateTemp(filepath.Dir(cache), "thumb-*.tmp"); e == nil {
			name := f.Name()
			_, e = f.Write(data)
			closeErr := f.Close()
			if e == nil && closeErr == nil {
				_ = os.Rename(name, cache)
			}
			_ = os.Remove(name)
		}
	}
	return data, mime, nil
}

func (s *PhotoService) analyzeTagQueue(parent context.Context, dest string, photos []model.Photo) AnalyzeResult {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	s.batch.ResetStats()
	started := time.Now()
	var prepared, cached, succeeded atomic.Int64
	var phase atomic.Value
	phase.Store("กำลังตรวจรูปซ้ำและเตรียมคิว")
	snapshot := func() AnalysisMetrics {
		elapsed := time.Since(started).Seconds()
		return AnalysisMetrics{BatchStats: s.batch.Stats(), Phase: phase.Load().(string), Prepared: int(prepared.Load()), Cached: int(cached.Load()), ElapsedSeconds: elapsed, PhotosPerMinute: float64(succeeded.Load()) * 60 / max(0.001, elapsed)}
	}
	metricsDone := make(chan struct{})
	metricsStopped := make(chan struct{})
	go func() {
		defer close(metricsStopped)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-metricsDone:
				return
			case <-ticker.C:
				s.emit("analyze:metrics", snapshot())
			}
		}
	}()
	defer func() { close(metricsDone); <-metricsStopped }()

	outcomes := make(chan tagOutcome, s.batch.Settings().Workers*s.batch.Settings().Size)
	jobs := make(chan []tagGroup)
	var workers sync.WaitGroup
	version := "tags-caption-v2:" + s.batch.Model()
	for range s.batch.Settings().Workers {
		workers.Go(func() {
			for groups := range jobs {
				if ctx.Err() != nil {
					return
				}
				s.processTagGroups(ctx, dest, version, groups, outcomes)
			}
		})
	}
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		defer close(jobs)
		groups := []tagGroup{}
		index := map[string]int{}
		for _, p := range photos {
			if ctx.Err() != nil {
				return
			}
			if p.FileName != filepath.Base(p.FileName) {
				outcomes <- tagOutcome{photo: p, err: errors.New("ชื่อไฟล์ไม่ถูกต้อง")}
				continue
			}
			hash, err := analysisHash(ctx, filepath.Join(dest, p.FileName))
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				outcomes <- tagOutcome{photo: p, err: err}
				continue
			}
			prepared.Add(1)
			if i, ok := index[hash]; ok {
				groups[i].photos = append(groups[i].photos, p)
			} else {
				index[hash] = len(groups)
				groups = append(groups, tagGroup{hash: hash, photos: []model.Photo{p}})
			}
		}
		phase.Store("กำลังติดแท็กเป็นชุด")
		for i := 0; i < len(groups); i += s.batch.Settings().Size {
			end := min(len(groups), i+s.batch.Settings().Size)
			select {
			case <-ctx.Done():
				return
			case jobs <- groups[i:end]:
			}
		}
	}()
	go func() { <-producerDone; workers.Wait(); close(outcomes) }()
	result := AnalyzeResult{Items: []FileResult{}}
	for out := range outcomes {
		item := FileResult{Name: out.photo.FileName, Status: StatusMoved}
		fatal := false
		if out.err == nil {
			result.Done++
			succeeded.Add(1)
			if out.cached {
				cached.Add(1)
			}
		} else if errors.Is(out.err, context.Canceled) && ctx.Err() != nil {
			item.Status = "cancelled"
			item.Message = "ยกเลิกแล้ว งานยังอยู่ในคิว"
		} else {
			result.Failed++
			item.Status = StatusFailed
			item.Message = out.err.Error()
			var api *gemini.APIError
			fatal = errors.As(out.err, &api) && (api.Fatal() || api.Status == 429)
			if fatal {
				cancel()
			}
		}
		// Save pending/error state for non-successful items; completed tags are already
		// committed atomically by the worker before a success reaches this channel.
		if out.err != nil {
			state := "retry"
			if item.Status == "cancelled" {
				state = "pending"
			}
			s.analysisSaveMu.Lock()
			err := s.photos.AnalysisState(out.photo.ID, state, item.Message, false)
			s.analysisSaveMu.Unlock()
			if err != nil {
				item.Message += "; บันทึกสถานะคิวไม่ได้: " + err.Error()
				fatal = true
				cancel()
			}
		}
		result.Items = append(result.Items, item)
		s.emit(analyzeProgress, analysisProgressEvent{ProgressEvent: ProgressEvent{Done: len(result.Items), Total: len(photos), Name: item.Name, Status: item.Status}, Succeeded: result.Done, Failed: result.Failed, Message: item.Message, Stopped: fatal})
	}
	result.Cancelled = parent.Err() != nil
	phase.Store("จบรอบการติดแท็ก")
	result.Metrics = snapshot()
	s.emit("analyze:metrics", result.Metrics)
	// Summary contains counters and model only, never filenames, images or keys.
	if os.MkdirAll("logs", 0700) == nil {
		if f, err := os.OpenFile(filepath.Join("logs", "tag-runs.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
			_ = json.NewEncoder(f).Encode(map[string]any{"time": time.Now().UTC(), "model": s.batch.Model(), "batch_size": s.batch.Settings().Size, "done": result.Done, "failed": result.Failed, "cancelled": result.Cancelled, "metrics": result.Metrics})
			_ = f.Close()
		}
	}
	return result
}

func (s *PhotoService) processTagGroups(ctx context.Context, dest, version string, groups []tagGroup, out chan<- tagOutcome) {
	pending := map[int64]tagGroup{}
	images := []gemini.TagImage{}
	complete := func(g tagGroup, result gemini.Result, err error, cache bool) {
		for i, p := range g.photos {
			saveErr := err
			if err == nil {
				s.analysisSaveMu.Lock()
				saveErr = s.photos.SaveTags(p.ID, result.Caption, result.Tags, g.hash, version)
				s.analysisSaveMu.Unlock()
			}
			out <- tagOutcome{photo: p, err: saveErr, cached: cache || i > 0}
		}
	}
	for _, g := range groups {
		if ctx.Err() != nil {
			complete(g, gemini.Result{}, ctx.Err(), false)
			continue
		}
		caption, tags, err := s.photos.CachedTags(g.hash, version)
		if err != nil {
			complete(g, gemini.Result{}, err, false)
			continue
		}
		if len(tags) > 0 {
			complete(g, gemini.Result{Caption: caption, Tags: tags}, nil, true)
			continue
		}
		p := g.photos[0]
		data, mime, err := tagImage(filepath.Join(dest, p.FileName), g.hash)
		if err != nil {
			complete(g, gemini.Result{}, err, false)
			continue
		}
		// Cap a raw unsupported-format image before base64 expansion; batches are
		// split below to stay under the request payload limit.
		if len(data) > 12<<20 {
			complete(g, gemini.Result{}, errors.New("ภาพใหญ่เกินไป กรุณาแปลง HEIC/WebP เป็น JPEG ก่อน"), false)
			continue
		}
		pending[p.ID] = g
		images = append(images, gemini.TagImage{ID: p.ID, Data: data, MIME: mime})
	}
	// Bound total request size even when HEIC/WebP cannot be thumbnailed.
	for len(images) > 0 {
		bytes, end := 0, 0
		for end < len(images) && bytes+len(images[end].Data) <= 12<<20 {
			bytes += len(images[end].Data)
			end++
		}
		if end == 0 {
			end = 1
		}
		chunk := images[:end]
		images = images[end:]
		var lastErr error
		for attempt := 0; attempt < 3 && len(chunk) > 0; attempt++ {
			if ctx.Err() != nil {
				lastErr = ctx.Err()
				break
			}
			stateErr := error(nil)
			for _, im := range chunk {
				for _, p := range pending[im.ID].photos {
					s.analysisSaveMu.Lock()
					err := s.photos.AnalysisState(p.ID, "running", "", true)
					s.analysisSaveMu.Unlock()
					if err != nil {
						stateErr = err
					}
				}
			}
			if stateErr != nil {
				lastErr = stateErr
				break
			}
			results, err := s.batch.Tag(ctx, chunk)
			lastErr = err
			remaining := make([]gemini.TagImage, 0, len(chunk))
			for _, im := range chunk {
				if tags, ok := results[im.ID]; ok {
					complete(pending[im.ID], tags, nil, false)
					delete(pending, im.ID)
				} else {
					remaining = append(remaining, im)
				}
			}
			chunk = remaining
			if len(chunk) == 0 {
				break
			}
			if lastErr == nil {
				lastErr = errors.New("ไม่มีแท็กสำหรับบางภาพ")
			}
			var api *gemini.APIError
			if errors.As(lastErr, &api) && api.Fatal() {
				break
			}
			if attempt < 2 {
				for _, im := range chunk {
					for _, p := range pending[im.ID].photos {
						s.analysisSaveMu.Lock()
						e := s.photos.AnalysisState(p.ID, "retry", lastErr.Error(), false)
						s.analysisSaveMu.Unlock()
						if e != nil {
							lastErr = e
							stateErr = e
						}
					}
				}
				if stateErr != nil {
					break
				}
				timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					lastErr = ctx.Err()
				case <-timer.C:
				}
			}
		}
		for _, im := range chunk {
			complete(pending[im.ID], gemini.Result{}, lastErr, false)
			delete(pending, im.ID)
		}
	}
}

// Description is an explicit, separate request. Existing tags are kept intact.
func (s *PhotoService) DescribePhoto(id int64) (string, error) {
	if !s.opMu.TryLock() {
		return "", errBusy
	}
	defer s.opMu.Unlock()
	p, err := s.photos.PhotoByID(s.snapshot().Dest, id)
	if err != nil {
		return "", err
	}
	if p.Caption != "" {
		return p.Caption, nil
	}
	if p.FileName != filepath.Base(p.FileName) {
		return "", errors.New("ชื่อไฟล์ไม่ถูกต้อง")
	}
	data, mime, err := analysisImage(filepath.Join(p.Dest, p.FileName))
	if err != nil {
		return "", err
	}
	base := s.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 90*time.Second)
	defer cancel()
	modelName := os.Getenv("GEMINI_TAG_MODEL")
	if modelName == "" {
		modelName = gemini.DefaultTagModel
	}
	result, err := gemini.New(os.Getenv("GEMINI_API_KEY"), modelName).Describe(ctx, data, mime)
	if err != nil {
		return "", err
	}
	if err = s.photos.SaveCaption(id, result.Caption); err != nil {
		return "", fmt.Errorf("บันทึกคำอธิบายไม่ได้: %w", err)
	}
	return result.Caption, nil
}
