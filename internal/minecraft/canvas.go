package minecraft

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	nativewebp "github.com/HugoSmits86/nativewebp"
)

// Canvas 是 GD 风格画布的最小子集：内部像素为 ARGB（0 全透明，255 不透明）。
type Canvas struct {
	Width  int
	Height int
	Pixels []uint32
}

// NewCanvas 创建透明画布（对应 GD 的 fill white + alpha=127）。
func NewCanvas(width, height int) *Canvas {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	pixels := make([]uint32, width*height)
	for i := range pixels {
		pixels[i] = 0x00FFFFFF
	}
	return &Canvas{Width: width, Height: height, Pixels: pixels}
}

// CanvasFromPNG 解码 PNG 为画布；失败返回错误。
func CanvasFromPNG(data []byte) (*Canvas, error) {
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		img, _, err2 := image.Decode(bytes.NewReader(data))
		if err2 != nil {
			return nil, err
		}
		decoded = img
	}
	bounds := decoded.Bounds()
	canvas := NewCanvas(bounds.Dx(), bounds.Dy())
	for y := 0; y < canvas.Height; y++ {
		for x := 0; x < canvas.Width; x++ {
			pixel := color.NRGBAModel.Convert(decoded.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			canvas.Pixels[y*canvas.Width+x] = uint32(pixel.A)<<24 |
				uint32(pixel.R)<<16 | uint32(pixel.G)<<8 | uint32(pixel.B)
		}
	}
	return canvas, nil
}

// image 把画布转换为标准 image.Image。
func (c *Canvas) image() *image.NRGBA {
	output := image.NewNRGBA(image.Rect(0, 0, c.Width, c.Height))
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			argb := c.Pixels[y*c.Width+x]
			offset := y*output.Stride + x*4
			output.Pix[offset] = byte(argb >> 16)
			output.Pix[offset+1] = byte(argb >> 8)
			output.Pix[offset+2] = byte(argb)
			output.Pix[offset+3] = byte(argb >> 24)
		}
	}
	return output
}

// PNG 编码为 PNG 字节。
func (c *Canvas) PNG() ([]byte, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, c.image()); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WebP 编码为 WebP 字节（对应 PHP GD 的 IMG_WEBP 输出）。
func (c *Canvas) WebP() ([]byte, error) {
	var buffer bytes.Buffer
	if err := nativewebp.Encode(&buffer, c.image(), nil); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// PixelAt 读取像素（越界按边缘裁剪）。
func (c *Canvas) PixelAt(x, y int) uint32 {
	return c.Pixels[clamp(y, 0, c.Height-1)*c.Width+clamp(x, 0, c.Width-1)]
}

func (c *Canvas) setPixel(x, y int, argb uint32) {
	if x < 0 || y < 0 || x >= c.Width || y >= c.Height {
		return
	}
	c.Pixels[y*c.Width+x] = argb
}

// blendPixel 对应 imagecopy 的单像素 source-over 语义。
func (c *Canvas) blendPixel(x, y int, argb uint32) {
	alpha := int(argb >> 24)
	if alpha <= 0 {
		return
	}
	if alpha == 255 {
		c.setPixel(x, y, argb)
		return
	}
	c.blend(x, y, argb, alpha)
}

func (c *Canvas) blend(x, y int, argb uint32, alpha int) {
	if x < 0 || y < 0 || x >= c.Width || y >= c.Height {
		return
	}
	index := y*c.Width + x
	dst := c.Pixels[index]
	sa := float64(alpha) / 255
	da := float64(dst>>24) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		c.Pixels[index] = 0
		return
	}
	channel := func(shift uint) uint32 {
		s := float64((argb>>shift)&0xFF) / 255
		d := float64((dst>>shift)&0xFF) / 255
		return uint32(math.Round((((s * sa) + (d * da * (1 - sa))) / outA) * 255))
	}
	c.Pixels[index] = uint32(math.Round(outA*255))<<24 |
		channel(16)<<16 | channel(8)<<8 | channel(0)
}

// colorAtGD 把 ARGB 转成 GD int（gdAlpha 0=不透明，127=透明）。
func (c *Canvas) colorAtGD(x, y int) uint32 {
	argb := c.PixelAt(x, y)
	gdAlpha := 127 - int(math.Round(float64((argb>>24)*127)/255.0))
	return uint32(gdAlpha)<<24 | (argb & 0x00FFFFFF)
}

func alpha255(gdColor uint32) int {
	gdAlpha := int((gdColor >> 24) & 0x7F)
	return int(math.Round(float64(127-gdAlpha) * 255.0 / 127))
}

func toARGB(gdColor uint32) uint32 {
	return uint32(alpha255(gdColor))<<24 | (gdColor & 0x00FFFFFF)
}

// FilledRectangle 对应 imagefilledrectangle（源覆盖合成）。
func (c *Canvas) FilledRectangle(x1, y1, x2, y2 int, gdColor uint32) {
	alpha := alpha255(gdColor)
	if alpha <= 0 {
		return
	}
	left, right := minInt(x1, x2), maxInt(x1, x2)
	top, bottom := minInt(y1, y2), maxInt(y1, y2)
	argb := toARGB(gdColor)
	for y := maxInt(0, top); y <= minInt(c.Height-1, bottom); y++ {
		for x := maxInt(0, left); x <= minInt(c.Width-1, right); x++ {
			c.blend(x, y, argb, alpha)
		}
	}
}

// FilledPolygon 对应 imagefilledpolygon（扫描线、无抗锯齿）。
func (c *Canvas) FilledPolygon(points []float64, gdColor uint32) {
	alpha := alpha255(gdColor)
	count := len(points) / 2
	if alpha <= 0 || count < 3 {
		return
	}
	xs := make([]int, count)
	ys := make([]int, count)
	minY, maxY := int(^uint(0)>>1), -int(^uint(0)>>1)-1
	for i := 0; i < count; i++ {
		xs[i] = int(math.Round(points[i*2]))
		ys[i] = int(math.Round(points[i*2+1]))
		if ys[i] < minY {
			minY = ys[i]
		}
		if ys[i] > maxY {
			maxY = ys[i]
		}
	}
	argb := toARGB(gdColor)
	start := maxInt(0, minY)
	end := minInt(c.Height-1, maxY)
	for y := start; y <= end; y++ {
		crossings := make([]float64, 0, count)
		for i := 0; i < count; i++ {
			j := (i + 1) % count
			y1, y2 := ys[i], ys[j]
			if y1 == y2 {
				continue
			}
			scan := float64(y) + 0.5
			if (float64(y1) <= scan && float64(y2) > scan) ||
				(float64(y2) <= scan && float64(y1) > scan) {
				x := float64(xs[i]) + (scan-float64(y1))*float64(xs[j]-xs[i])/float64(y2-y1)
				crossings = append(crossings, x)
			}
		}
		sortFloats(crossings)
		for k := 0; k+1 < len(crossings); k += 2 {
			from := maxInt(0, int(math.Ceil(crossings[k])))
			to := minInt(c.Width-1, int(math.Floor(crossings[k+1])))
			for x := from; x <= to; x++ {
				c.blend(x, y, argb, alpha)
			}
		}
	}
}

// ResampleRegion 对应 imagecopyresampled（双线性）。
func (c *Canvas) ResampleRegion(source *Canvas, dstX, dstY, dstW, dstH,
	srcX, srcY, srcW, srcH int) {
	if dstW <= 0 || dstH <= 0 || srcW <= 0 || srcH <= 0 {
		return
	}
	for y := 0; y < dstH; y++ {
		sy := float64(srcY) + (float64(y)+0.5)*float64(srcH)/float64(dstH)
		y0 := clamp(int(math.Floor(sy-0.5)), 0, source.Height-1)
		y1 := minInt(y0+1, source.Height-1)
		fy := clampFloat(sy-0.5-float64(y0), 0, 1)
		for x := 0; x < dstW; x++ {
			sx := float64(srcX) + (float64(x)+0.5)*float64(srcW)/float64(dstW)
			x0 := clamp(int(math.Floor(sx-0.5)), 0, source.Width-1)
			x1 := minInt(x0+1, source.Width-1)
			fx := clampFloat(sx-0.5-float64(x0), 0, 1)
			argb := bilinear(source.PixelAt(x0, y0), source.PixelAt(x1, y0),
				source.PixelAt(x0, y1), source.PixelAt(x1, y1), fx, fy)
			c.setPixel(dstX+x, dstY+y, argb)
		}
	}
}

// Resize 双线性缩放到新画布。
func (c *Canvas) Resize(width, height int) *Canvas {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	output := NewCanvas(width, height)
	output.ResampleRegion(c, 0, 0, width, height, 0, 0, c.Width, c.Height)
	return output
}

// CopyFrom 整块直拷（源覆盖）。
func (c *Canvas) CopyFrom(source *Canvas, dstX, dstY int) {
	for y := 0; y < source.Height; y++ {
		for x := 0; x < source.Width; x++ {
			argb := source.PixelAt(x, y)
			alpha := int(argb >> 24)
			if alpha == 0 {
				continue
			}
			if alpha == 255 {
				c.setPixel(dstX+x, dstY+y, argb)
			} else {
				c.blend(dstX+x, dstY+y, argb, alpha)
			}
		}
	}
}

func bilinear(p00, p10, p01, p11 uint32, fx, fy float64) uint32 {
	channel := func(shift uint) uint32 {
		c00 := float64((p00 >> shift) & 0xFF)
		c10 := float64((p10 >> shift) & 0xFF)
		c01 := float64((p01 >> shift) & 0xFF)
		c11 := float64((p11 >> shift) & 0xFF)
		top := c00 + (c10-c00)*fx
		bottom := c01 + (c11-c01)*fx
		return uint32(math.Round(top + (bottom-top)*fy))
	}
	return channel(24)<<24 | channel(16)<<16 | channel(8)<<8 | channel(0)
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sortFloats(values []float64) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
