package captcha

import (
	"bytes"
	"image/jpeg"
	"testing"
)

func TestGenerate(t *testing.T) {
	phrase, data, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(phrase) != 5 {
		t.Fatalf("phrase=%q", phrase)
	}
	if len(data) == 0 {
		t.Fatal("empty image")
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 34 {
		t.Fatalf("size=%v", img.Bounds())
	}
}
