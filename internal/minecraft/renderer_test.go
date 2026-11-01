package minecraft

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"neoskin/internal/resources"
)

func TestRenderAvatarsAndPreview(t *testing.T) {
	skin := resources.DefaultSkin()

	avatar2d, err := Render2dAvatar(skin, 25)
	if err != nil {
		t.Fatal(err)
	}
	if avatar2d.Width != 225 || avatar2d.Height != 225 {
		t.Fatalf("2d avatar=%dx%d", avatar2d.Width, avatar2d.Height)
	}

	avatar3d, err := Render3dAvatar(skin, 25)
	if err != nil {
		t.Fatal(err)
	}
	if avatar3d.Width != 318 || avatar3d.Height != 338 {
		t.Fatalf("3d avatar=%dx%d", avatar3d.Width, avatar3d.Height)
	}

	preview, err := RenderSkin(skin, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Width != 430 || preview.Height != 437 {
		t.Fatalf("preview=%dx%d", preview.Width, preview.Height)
	}

	// 图像必须有非透明像素，避免渲染成空图。
	opaque := 0
	for _, pixel := range preview.Pixels {
		if pixel>>24 > 0 {
			opaque++
		}
	}
	if opaque < preview.Width*preview.Height/4 {
		t.Fatalf("preview looks empty: opaque=%d/%d", opaque, len(preview.Pixels))
	}
}

func TestRenderCape(t *testing.T) {
	cape := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			cape.Set(x, y, color.NRGBA{R: 200, G: 30, B: 40, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, cape); err != nil {
		t.Fatal(err)
	}

	canvas, err := RenderCape(buffer.Bytes(), 64)
	if err != nil {
		t.Fatal(err)
	}
	if canvas.Width != 120 || canvas.Height != 104 {
		t.Fatalf("cape=%dx%d", canvas.Width, canvas.Height)
	}
	if _, err := canvas.PNG(); err != nil {
		t.Fatal(err)
	}
	webpData, err := canvas.WebP()
	if err != nil {
		t.Fatal(err)
	}
	if len(webpData) < 12 || string(webpData[0:4]) != "RIFF" || string(webpData[8:12]) != "WEBP" {
		t.Fatalf("invalid webp header: %x", webpData[:12])
	}
}

func TestResizeAndBlend(t *testing.T) {
	canvas := NewCanvas(2, 2)
	canvas.setPixel(0, 0, 0xFFFF0000)
	scaled := canvas.Resize(4, 4)
	if scaled.Width != 4 || scaled.Height != 4 {
		t.Fatalf("scaled=%dx%d", scaled.Width, scaled.Height)
	}
	if scaled.PixelAt(0, 0)>>24 == 0 {
		t.Fatal("scaled pixel should be opaque")
	}
}

func TestCanvasFromPNGRejectsGarbage(t *testing.T) {
	if _, err := CanvasFromPNG([]byte("not a png")); err == nil {
		t.Fatal("expected decode error")
	}
}
