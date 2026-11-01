package captcha

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const Charset = "abcdefghijklmnpqrstuvwxyz123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Generate 返回 5 位验证码与 JPEG 图片（PHP 源返回 JPEG）。
func Generate() (string, []byte, error) {
	buffer := make([]byte, 5)
	if _, err := rand.Read(buffer); err != nil {
		return "", nil, err
	}

	phrase := make([]byte, 5)
	for i, value := range buffer {
		phrase[i] = Charset[int(value)%len(Charset)]
	}

	img := image.NewRGBA(image.Rect(0, 0, 100, 34))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{220, 220, 220, 255}},
		image.Point{}, draw.Src)

	face := basicfont.Face7x13
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{50, 50, 50, 255}),
		Face: face,
		Dot:  fixed.P(15, 22),
	}
	drawer.DrawString(string(phrase))

	var output bytes.Buffer
	if err := jpeg.Encode(&output, img, &jpeg.Options{Quality: 90}); err != nil {
		return "", nil, err
	}

	return string(phrase), output.Bytes(), nil
}
