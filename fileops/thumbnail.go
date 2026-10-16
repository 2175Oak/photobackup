package fileops

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // ลงทะเบียนตัวถอดรหัส GIF ให้ image.Decode
	"image/jpeg"
	_ "image/png" // ลงทะเบียนตัวถอดรหัส PNG ให้ image.Decode
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Thumbnail ย่อรูปให้ด้านที่ยาวที่สุดไม่เกิน maxSide พิกเซล แล้วคืนเป็นไฟล์ JPEG
// รองรับ JPEG, PNG, GIF (ไฟล์ชนิดอื่น เช่น HEIC หรือ WebP จะคืน error)
// หมุนรูปตามค่า EXIF Orientation ให้ด้วย เพราะรูปจากโทรศัพท์มักเก็บภาพตะแคงแล้วใช้ค่านี้บอกทิศ
//
// ใช้ส่งรูปตัวอย่างให้หน้าจอผ่าน method ของ Wails โดยตรง
// จึงไม่ต้องพึ่งเส้นทางไฟล์ของ asset server ซึ่งทำงานต่างกันระหว่าง wails dev และ wails build
func Thumbnail(path string, maxSide int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	orientation := 1
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".jpg" || ext == ".jpeg" {
		orientation = jpegOrientation(f)
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}

	src, _, err := image.Decode(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("แสดงตัวอย่างไฟล์นี้ไม่ได้: %w", err)
	}

	small := orient(shrink(src, maxSide), orientation)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// shrink ย่อรูปโดยสุ่มจุด 3x3 ในแต่ละช่วงของรูปต้นฉบับแล้วหาค่าเฉลี่ย
// (เร็วกว่าเฉลี่ยทุกพิกเซลมากสำหรับรูปความละเอียดสูง และพอสำหรับรูปตัวอย่าง)
// พื้นที่โปร่งใสจะถูกวางบนพื้นขาว เพราะผลลัพธ์เป็น JPEG ซึ่งไม่มีความโปร่งใส
func shrink(src image.Image, maxSide int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	scale := 1.0
	if long := max(w, h); long > maxSide {
		scale = float64(maxSide) / float64(long)
	}
	dw := max(1, int(float64(w)*scale+0.5))
	dh := max(1, int(float64(h)*scale+0.5))

	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	const n = 3
	for y := 0; y < dh; y++ {
		sy0 := b.Min.Y + y*h/dh
		sy1 := b.Min.Y + (y+1)*h/dh
		for x := 0; x < dw; x++ {
			sx0 := b.Min.X + x*w/dw
			sx1 := b.Min.X + (x+1)*w/dw

			var r, g, bl, count uint64
			for j := 0; j < n; j++ {
				sy := sy0 + (sy1-sy0)*(2*j+1)/(2*n)
				for i := 0; i < n; i++ {
					sx := sx0 + (sx1-sx0)*(2*i+1)/(2*n)
					pr, pg, pb, pa := src.At(sx, sy).RGBA() // คูณ alpha ไว้แล้ว ช่วง 0..0xffff
					r += uint64(pr + (0xffff - pa))
					g += uint64(pg + (0xffff - pa))
					bl += uint64(pb + (0xffff - pa))
					count++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8((r / count) >> 8),
				G: uint8((g / count) >> 8),
				B: uint8((bl / count) >> 8),
				A: 255,
			})
		}
	}
	return dst
}

// orient หมุน/พลิกรูปตามค่า EXIF Orientation (1-8) ให้ตรงกับที่ตาเห็น
func orient(img *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return img
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int // จุดในรูปต้นฉบับที่ต้องไปวางที่ (x, y)
			switch o {
			case 2: // พลิกซ้าย-ขวา
				sx, sy = w-1-x, y
			case 3: // หมุน 180 องศา
				sx, sy = w-1-x, h-1-y
			case 4: // พลิกบน-ล่าง
				sx, sy = x, h-1-y
			case 5: // transpose
				sx, sy = y, x
			case 6: // หมุน 90 องศาตามเข็มนาฬิกา
				sx, sy = y, h-1-x
			case 7: // transverse
				sx, sy = w-1-y, h-1-x
			case 8: // หมุน 90 องศาทวนเข็มนาฬิกา
				sx, sy = w-1-y, x
			}
			dst.SetRGBA(x, y, img.RGBAAt(sx, sy))
		}
	}
	return dst
}

// jpegOrientation อ่านค่า EXIF Orientation จากไฟล์ JPEG โดยไม่ต้องถอดรหัสทั้งรูป
// คืน 1 (ไม่ต้องหมุน) ถ้าไม่มี EXIF หรืออ่านไม่ได้
func jpegOrientation(r io.Reader) int {
	br := bufio.NewReader(r)

	var soi [2]byte
	if _, err := io.ReadFull(br, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return 1
	}
	for {
		b, err := br.ReadByte()
		if err != nil || b != 0xFF {
			return 1
		}
		marker, err := br.ReadByte()
		if err != nil {
			return 1
		}
		for marker == 0xFF { // ไบต์ 0xFF ที่เติมไว้ก่อน marker
			if marker, err = br.ReadByte(); err != nil {
				return 1
			}
		}
		if marker == 0xDA || marker == 0xD9 { // เริ่มข้อมูลรูป หรือจบไฟล์ ไม่พบ EXIF
			return 1
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) { // marker ที่ไม่มีความยาว
			continue
		}

		var l [2]byte
		if _, err := io.ReadFull(br, l[:]); err != nil {
			return 1
		}
		size := int(binary.BigEndian.Uint16(l[:])) - 2
		if size < 0 {
			return 1
		}
		if marker != 0xE1 { // ไม่ใช่ APP1 (ที่เก็บ EXIF) ข้ามไป
			if _, err := br.Discard(size); err != nil {
				return 1
			}
			continue
		}

		seg := make([]byte, size)
		if _, err := io.ReadFull(br, seg); err != nil {
			return 1
		}
		if len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
	}
}

// tiffOrientation หา tag Orientation (0x0112) ใน IFD0 ของข้อมูล TIFF ที่อยู่ใน EXIF
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(t[2:4]) != 42 {
		return 1
	}
	ifd := int(bo.Uint32(t[4:8]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	count := int(bo.Uint16(t[ifd : ifd+2]))
	for i := 0; i < count; i++ {
		e := ifd + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8 : e+10])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}
