package fileops

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsImage(t *testing.T) {
	cases := map[string]bool{
		"a.jpg": true, "a.JPEG": true, "photo.PNG": true, "x.heic": true, "x.webp": true,
		"a.txt": false, "noext": false, "._a.jpg": false, ".hidden.png": false, "a.jpg.part": false,
		"..": false,
	}
	for name, want := range cases {
		if got := IsImage(name); got != want {
			t.Errorf("IsImage(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestListImages(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.jpg"), "aaa")
	write(t, filepath.Join(dir, "B.PNG"), "bb")
	write(t, filepath.Join(dir, "note.txt"), "x")
	write(t, filepath.Join(dir, "._junk.jpg"), "x")
	if err := os.Mkdir(filepath.Join(dir, "folder.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ListImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "B.PNG" || names[1] != "a.jpg" {
		t.Fatalf("unexpected list: %v", names)
	}
	for _, f := range got {
		if f.Name == "a.jpg" && f.Size != 3 {
			t.Errorf("size of a.jpg = %d, want 3", f.Size)
		}
	}

	if _, err := ListImages(filepath.Join(dir, "missing")); err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestMoveFileSameVolume(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg")
	write(t, src, "hello")

	if err := MoveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if Exists(src) {
		t.Error("source should be gone after move")
	}
	if b, _ := os.ReadFile(dst); string(b) != "hello" {
		t.Errorf("dst content = %q", b)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "copy.jpg")
	write(t, src, "pixels")
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(src, old, old); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "pixels" {
		t.Errorf("dst content = %q", b)
	}
	if !Exists(src) {
		t.Error("CopyFile must not remove the source")
	}
	if fi, _ := os.Stat(dst); !fi.ModTime().Equal(old) {
		t.Errorf("mod time = %v, want %v", fi.ModTime(), old)
	}
	if Exists(dst + ".part") {
		t.Error("temporary .part file left behind")
	}

	// ต้นทางไม่มี ต้องไม่เหลือไฟล์ปลายทางหรือ .part
	missing := filepath.Join(dir, "nope.jpg")
	out := filepath.Join(dir, "out.jpg")
	if err := CopyFile(missing, out); err == nil {
		t.Error("expected error for missing source")
	}
	if Exists(out) || Exists(out+".part") {
		t.Error("failed copy must not leave files behind")
	}
}

// ทดสอบเส้นทาง fallback (rename ไม่ได้เพราะข้ามไดรฟ์ -> copy แล้วลบต้นทาง) บน Linux ที่มี /dev/shm คนละระบบไฟล์กับ /tmp
func TestMoveFileCrossDevice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cross-device test needs Linux /dev/shm")
	}
	shm, err := os.MkdirTemp("/dev/shm", "fileops-test-")
	if err != nil {
		t.Skip("no writable /dev/shm")
	}
	defer os.RemoveAll(shm)

	srcDir := t.TempDir()
	src, dst := filepath.Join(srcDir, "a.jpg"), filepath.Join(shm, "a.jpg")
	write(t, src, "cross-device")
	if os.Rename(src, dst) == nil {
		t.Skip("/tmp and /dev/shm are on the same filesystem here")
	}

	if err := MoveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if Exists(src) {
		t.Error("source should be removed after cross-device move")
	}
	if b, _ := os.ReadFile(dst); string(b) != "cross-device" {
		t.Errorf("dst content = %q", b)
	}
}

func TestSourceNotRemovedIsDetectable(t *testing.T) {
	err := errors.Join(ErrSourceNotRemoved, errors.New("x"))
	if !errors.Is(err, ErrSourceNotRemoved) {
		t.Error("errors.Is should match ErrSourceNotRemoved")
	}
}

func TestNormalizePath(t *testing.T) {
	dir := t.TempDir()

	a, err := NormalizePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{dir + string(filepath.Separator), `"` + dir + `"`, "  " + dir + "  ", filepath.Join(dir, "x", "..")} {
		got, err := NormalizePath(in)
		if err != nil || got != a {
			t.Errorf("NormalizePath(%q) = %q, %v; want %q", in, got, err, a)
		}
	}
	if _, err := NormalizePath("   "); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestSameDir(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	if !SameDir(dir, dir+string(filepath.Separator)) {
		t.Error("same directory with trailing separator should match")
	}
	if SameDir(dir, other) {
		t.Error("different directories should not match")
	}
	if SameDir("", dir) {
		t.Error("empty path never matches")
	}
}
