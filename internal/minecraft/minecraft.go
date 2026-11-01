package minecraft

// RenderSkin 对应 renderSkin($skin, $ratio, $isAlex)：正反两面并排。
func RenderSkin(skin []byte, ratio float64, isAlex bool) (*Canvas, error) {
	const vp, hp, ip = 15, 30, 15

	front, err := NewSkinRenderer(ratio, false, -45).Render(skin, isAlex)
	if err != nil {
		return nil, err
	}
	back, err := NewSkinRenderer(ratio, false, 135).Render(skin, isAlex)
	if err != nil {
		return nil, err
	}

	width, height := front.Width, front.Height
	canvas := NewCanvas((hp+width+ip)*2, vp*2+height)
	canvas.CopyFrom(back, hp, vp)
	canvas.CopyFrom(front, hp+(width+ip*2), vp)
	return canvas, nil
}

// RenderCape 对应 renderCape($cape, $height)。
func RenderCape(cape []byte, height int) (*Canvas, error) {
	const vp, hp = 20, 40

	resource, err := CanvasFromPNG(cape)
	if err != nil {
		return nil, err
	}
	srcWidth, srcHeight := resource.Width, resource.Height
	hdRatio := float64(srcWidth) / 64.0
	if height <= 0 {
		height = 200
	}
	outHeight := height
	outWidth := int(float64(outHeight) / 16.0 * 10)

	rendered := NewCanvas(outWidth, outHeight)
	rendered.ResampleRegion(resource, 0, 0, outWidth, outHeight,
		int(1*hdRatio), int(1*hdRatio),
		int(float64(srcWidth)*10/64.0), int(float64(srcHeight)*16/32.0))

	canvas := NewCanvas(hp*2+outWidth, vp*2+outHeight)
	canvas.CopyFrom(rendered, hp, vp)
	return canvas, nil
}

// Render2dAvatar 对应 render2dAvatar($skin, $ratio)。
func Render2dAvatar(skin []byte, ratio float64) (*Canvas, error) {
	return NewSkinRendererV(ratio, true, 0, 0).Render(skin, false)
}

// Render3dAvatar 对应 render3dAvatar($skin, $ratio)（hR=45，vR 默认 -25）。
func Render3dAvatar(skin []byte, ratio float64) (*Canvas, error) {
	return NewSkinRenderer(ratio, true, 45).Render(skin, false)
}
