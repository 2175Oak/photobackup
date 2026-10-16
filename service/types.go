package service

// สถานะของแต่ละไฟล์ที่ผ่านการย้าย/ลบ
const (
	StatusMoved   = "moved"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
	StatusDeleted = "deleted"
)

// State คือโฟลเดอร์ต้นทาง/ปลายทางที่ใช้อยู่ตอนนี้
type State struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// FileInfo คือรูปหนึ่งรูปในโฟลเดอร์ (InDest = มีอยู่ในปลายทางหรือฐานข้อมูลแล้ว)
type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	InDest  bool   `json:"inDest"`
}

type SourceInfo struct {
	Path    string `json:"path"`
	Count   int    `json:"count"`
	Warning string `json:"warning"`
}

// IntegrityItem บอกว่ารายการในฐานข้อมูลยังมีไฟล์จริงอยู่ที่ปลายทางหรือไม่
type IntegrityItem struct {
	Filename string `json:"filename"`
	OK       bool   `json:"ok"`
}

type DestInfo struct {
	Path      string          `json:"path"`
	Integrity []IntegrityItem `json:"integrity"`
	Warning   string          `json:"warning"`
}

type FileResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type MoveResult struct {
	Moved   int          `json:"moved"`
	Skipped int          `json:"skipped"`
	Failed  int          `json:"failed"`
	Seconds float64      `json:"seconds"`
	Items   []FileResult `json:"items"`
}

type DeleteResult struct {
	Deleted int          `json:"deleted"`
	Failed  int          `json:"failed"`
	Items   []FileResult `json:"items"`
}

type CleanAllResult struct {
	Photos   int64 `json:"photos"`
	Tags     int64 `json:"tags"`
	Settings int64 `json:"settings"`
}

// ProgressEvent ถูกส่งไปหน้าจอผ่านอีเวนต์ "move:progress" ระหว่างย้ายไฟล์
type ProgressEvent struct {
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
