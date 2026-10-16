package fileops

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("thumbnail is not a valid JPEG: %v", err)
	}
	return img
}

func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 120, 255})
		}
	}
	return img
}

func writeJPEG(t *testing.T, path string, img image.Image, exifOrientation int) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if exifOrientation != 0 {
		// แทรก APP1 (EXIF) ที่มีแค่ tag Orientation ต่อจาก SOI
		tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, byte(exifOrientation), 0, 0, 0, 0, 0, 0}
		payload := append([]byte("Exif\x00\x00"), tiff...)
		seg := []byte{0xFF, 0xE1, 0, 0}
		binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
		seg = append(seg, payload...)
		data = append(append(append([]byte{}, data[:2]...), seg...), data[2:]...)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestThumbnailShrinksKeepingAspect(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.jpg")
	writeJPEG(t, p, gradient(1000, 500), 0)

	data, err := Thumbnail(p, 100)
	if err != nil {
		t.Fatal(err)
	}
	b := decodeJPEG(t, data).Bounds()
	if b.Dx() != 100 || b.Dy() != 50 {
		t.Errorf("size = %dx%d, want 100x50", b.Dx(), b.Dy())
	}
}

func TestThumbnailDoesNotUpscale(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "small.jpg")
	writeJPEG(t, p, gradient(40, 20), 0)

	data, err := Thumbnail(p, 300)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, data).Bounds(); b.Dx() != 40 || b.Dy() != 20 {
		t.Errorf("size = %dx%d, want 40x20", b.Dx(), b.Dy())
	}
}

func TestThumbnailAppliesExifOrientation(t *testing.T) {
	dir := t.TempDir()
	// รูปจากโทรศัพท์ถ่ายแนวตั้ง: เก็บเป็นภาพกว้าง 40x20 แต่ EXIF บอกให้หมุน 90 องศา
	rotated := filepath.Join(dir, "portrait.jpg")
	writeJPEG(t, rotated, gradient(40, 20), 6)
	data, err := Thumbnail(rotated, 100)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, data).Bounds(); b.Dx() != 20 || b.Dy() != 40 {
		t.Errorf("orientation 6: size = %dx%d, want 20x40", b.Dx(), b.Dy())
	}

	// Orientation 1 หรือไม่มี EXIF ต้องไม่หมุน
	plain := filepath.Join(dir, "plain.jpg")
	writeJPEG(t, plain, gradient(40, 20), 1)
	data, err = Thumbnail(plain, 100)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, data).Bounds(); b.Dx() != 40 || b.Dy() != 20 {
		t.Errorf("orientation 1: size = %dx%d, want 40x20", b.Dx(), b.Dy())
	}
}

func TestOrientMapsPixels(t *testing.T) {
	// รูป 3x2 ที่แต่ละพิกเซลมีค่า R = 10*x + y ไม่ซ้ำกัน
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			src.SetRGBA(x, y, color.RGBA{uint8(10*x + y), 0, 0, 255})
		}
	}
	at := func(img *image.RGBA, x, y int) int { return int(img.RGBAAt(x, y).R) }

	// หมุน 90 องศาตามเข็ม (6): มุมซ้ายบนเดิมต้องไปอยู่มุมขวาบน
	r := orient(src, 6)
	if r.Bounds().Dx() != 2 || r.Bounds().Dy() != 3 {
		t.Fatalf("o=6 size = %v", r.Bounds())
	}
	if at(r, 1, 0) != at(src, 0, 0) || at(r, 0, 0) != at(src, 0, 1) {
		t.Errorf("o=6 pixels wrong: top-left=%d top-right=%d", at(r, 0, 0), at(r, 1, 0))
	}

	// หมุน 180 องศา (3)
	r = orient(src, 3)
	if at(r, 0, 0) != at(src, 2, 1) || at(r, 2, 1) != at(src, 0, 0) {
		t.Errorf("o=3 pixels wrong")
	}

	// หมุน 90 องศาทวนเข็ม (8): มุมซ้ายบนเดิมต้องไปอยู่มุมซ้ายล่าง
	r = orient(src, 8)
	if r.Bounds().Dx() != 2 || r.Bounds().Dy() != 3 {
		t.Fatalf("o=8 size = %v", r.Bounds())
	}
	if at(r, 0, 2) != at(src, 0, 0) {
		t.Errorf("o=8: bottom-left=%d want %d", at(r, 0, 2), at(src, 0, 0))
	}

	// พลิกซ้าย-ขวา (2)
	r = orient(src, 2)
	if at(r, 0, 0) != at(src, 2, 0) {
		t.Errorf("o=2 pixels wrong")
	}

	// ค่านอกช่วงต้องคืนรูปเดิม
	if orient(src, 1) != src || orient(src, 9) != src {
		t.Error("o=1 and invalid values must return the same image")
	}
}

func TestThumbnailPNGTransparencyBecomesWhite(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8)) // ทุกพิกเซลโปร่งใสทั้งหมด
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "clear.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := Thumbnail(p, 64)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decodeJPEG(t, data).At(4, 4).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("transparent pixel = (%d,%d,%d), want near white", r>>8, g>>8, b>>8)
	}
}

func TestThumbnailUnsupportedFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake.heic")
	write(t, p, "not an image")
	if _, err := Thumbnail(p, 100); err == nil {
		t.Error("expected an error for an undecodable file")
	}
	if _, err := Thumbnail(filepath.Join(dir, "missing.jpg"), 100); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestJPEGOrientationWithoutExif(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "noexif.jpg")
	writeJPEG(t, p, gradient(10, 10), 0)
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := jpegOrientation(f); got != 1 {
		t.Errorf("orientation = %d, want 1", got)
	}
}
