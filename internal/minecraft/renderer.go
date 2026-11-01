package minecraft

import "math"

// 与 Blessing\Renderer\SkinRenderer 对齐的字段。
var (
	allFaceNames = []string{"back", "right", "top", "front", "left", "bottom"}
	memberNames  = []string{"head", "torso", "rightArm", "leftArm", "rightLeg", "leftLeg"}
	pieceNames   = []string{"helmet", "head", "torso", "rightArm", "leftArm", "rightLeg", "leftLeg"}
)

type cubePoint struct {
	point *Point3
	faces []string
}

type drawPass struct {
	piece string
	faces []string
}

type angles struct {
	cosAlpha float64
	sinAlpha float64
	cosOmega float64
	sinOmega float64
}

type facePair struct {
	back  []string
	front []string
}

// SkinRenderer 是 Blessing\Renderer\SkinRenderer 的移植。
type SkinRenderer struct {
	ratio    float64
	headOnly bool
	layers   bool

	hR, vR float64
	hrh    float64
	vrll   float64
	vrrl   float64
	vrla   float64
	vrra   float64

	skin          *Canvas
	isNewSkinType bool
	isAlex        bool
	hd            float64

	alpha, omega       float64
	cosAlpha, sinAlpha float64
	cosOmega, sinOmega float64

	membersAngles map[string]angles
	bounds        Bounds

	allFaces   []string
	frontFaces []string
	backFaces  []string

	visibleFaces map[string]facePair
	cubePoints   []cubePoint

	polygons     map[string]map[string][]*Polygon
	volumePoints map[string]*Point3
}

// NewSkinRenderer 对应 new SkinRenderer($ratio, $headOnly, $horizontalRotation)。
func NewSkinRenderer(ratio float64, headOnly bool, horizontalRotation float64) *SkinRenderer {
	return NewSkinRendererFull(ratio, headOnly, horizontalRotation, -25, 0, 0, 0, 0, 0, true)
}

// NewSkinRendererV 对应四参构造（verticalRotation 显式给出）。
func NewSkinRendererV(ratio float64, headOnly bool, horizontalRotation, verticalRotation float64) *SkinRenderer {
	return NewSkinRendererFull(ratio, headOnly, horizontalRotation, verticalRotation,
		0, 0, 0, 0, 0, true)
}

// NewSkinRendererFull 对应 Java/PHP 的全参数构造。
func NewSkinRendererFull(ratio float64, headOnly bool, hR, vR, hrh, vrll, vrrl,
	vrla, vrra float64, layers bool) *SkinRenderer {
	return &SkinRenderer{
		ratio: ratio, headOnly: headOnly, layers: layers,
		hR: hR, vR: vR, hrh: hrh,
		vrll: vrll, vrrl: vrrl, vrla: vrla, vrra: vrra,
	}
}

// Render 对应 render()。
func (r *SkinRenderer) Render(source []byte, isAlex bool) (*Canvas, error) {
	skin, err := CanvasFromPNG(source)
	if err != nil {
		return nil, err
	}
	if skin.Width > 256 {
		newHeight := int(float64(skin.Height) / (float64(skin.Width) / 256.0))
		dest := NewCanvas(256, newHeight)
		dest.ResampleRegion(skin, 0, 0, 256, newHeight, 0, 0, skin.Width, skin.Height)
		skin = dest
	}
	r.skin = skin
	r.isAlex = isAlex
	r.hd = float64(skin.Width) / 64.0
	r.isNewSkinType = skin.Width == skin.Height

	r.makeBackgroundTransparent()
	if r.layers {
		r.fixNewSkinTypeLayers()
	}
	r.calculateAngles()
	r.facesDetermination()
	r.generatePolygons()
	r.memberRotation()
	r.createProjectionPlan()
	image := r.displayImage()
	r.skin = nil
	return image, nil
}

func (r *SkinRenderer) makeBackgroundTransparent() {
	skin := r.skin
	tempValue := int64(math.MinInt64)
	needRemove := true

	for iH := 0; iH < 8; iH++ {
		for iV := 0; iV < 8; iV++ {
			pixelColor := skin.colorAtGD(iH, iV)
			gdAlpha := int((pixelColor >> 24) & 0x7F)
			if gdAlpha > 120 {
				needRemove = false
			}
			if tempValue == math.MinInt64 {
				tempValue = int64(pixelColor)
			} else if tempValue != int64(pixelColor) {
				needRemove = false
			}
		}
	}

	imgX, imgY := skin.Width, skin.Height
	dst := NewCanvas(imgX, imgY)
	var fillGd uint32
	bgRgb := uint32(0)
	if needRemove {
		bgRgb = uint32(tempValue) & 0x00FFFFFF
		fillGd = bgRgb
	} else {
		fillGd = 0
	}

	positionMultiply := float64(imgX) / 64.0
	rectF := func(x1, y1, x2, y2 int, color uint32) {
		dst.FilledRectangle(int(float64(x1)*positionMultiply), int(float64(y1)*positionMultiply),
			int(float64(x2)*positionMultiply), int(float64(y2)*positionMultiply), color)
	}
	rectF(8, 0, 23, 7, fillGd)
	rectF(0, 8, 31, 15, fillGd)
	rectF(4, 16, 11, 19, fillGd)
	rectF(20, 16, 35, 19, fillGd)
	rectF(44, 16, 51, 19, fillGd)
	rectF(0, 20, 54, 31, fillGd)
	rectF(20, 48, 27, 51, fillGd)
	rectF(36, 48, 43, 51, fillGd)
	rectF(16, 52, 47, 63, fillGd)

	for y := 0; y < imgY; y++ {
		for x := 0; x < imgX; x++ {
			src := skin.PixelAt(x, y)
			if needRemove && (src&0x00FFFFFF) == bgRgb {
				continue
			}
			srcGdAlpha := 127 - javaRound(float32((src>>24)&0xFF)*127/255)
			if srcGdAlpha >= 127 {
				continue
			}
			dst.blendPixel(x, y, src)
		}
	}
	r.skin = dst
}

func javaRound(value float32) int {
	return int(math.Floor(float64(value) + 0.5))
}

func (r *SkinRenderer) fixNewSkinTypeLayers() {
	if !r.isNewSkinType {
		return
	}
	copySelf(r.skin, 0, 16, 0, 32, 56, 16)
	copySelf(r.skin, 16, 48, 0, 48, 16, 16)
	copySelf(r.skin, 32, 48, 48, 48, 16, 16)
}

func copySelf(img *Canvas, dx, dy, sx, sy, w, h int) {
	type pixel struct {
		x, y  int
		color uint32
	}
	snapshot := make([]pixel, 0, w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			snapshot = append(snapshot, pixel{dx + i, dy + j, img.PixelAt(sx+i, sy+j)})
		}
	}
	for _, item := range snapshot {
		img.blendPixel(item.x, item.y, item.color)
	}
}

func (r *SkinRenderer) calculateAngles() {
	r.alpha = r.vR * math.Pi / 180
	r.omega = r.hR * math.Pi / 180
	r.cosAlpha = math.Cos(r.alpha)
	r.sinAlpha = math.Sin(r.alpha)
	r.cosOmega = math.Cos(r.omega)
	r.sinOmega = math.Sin(r.omega)

	r.membersAngles = map[string]angles{"torso": anglesOf(0, 0)}
	head := anglesOf(0, r.hrh*math.Pi/180)
	r.membersAngles["head"] = head
	r.membersAngles["helmet"] = head
	r.membersAngles["rightArm"] = anglesOf(r.vrra*math.Pi/180, 0)
	r.membersAngles["leftArm"] = anglesOf(r.vrla*math.Pi/180, 0)
	r.membersAngles["rightLeg"] = anglesOf(r.vrrl*math.Pi/180, 0)
	r.membersAngles["leftLeg"] = anglesOf(r.vrll*math.Pi/180, 0)
	r.bounds = Bounds{}
}

func anglesOf(alphaRad, omegaRad float64) angles {
	return angles{math.Cos(alphaRad), math.Sin(alphaRad), math.Cos(omegaRad), math.Sin(omegaRad)}
}

func (r *SkinRenderer) facesDetermination() {
	ca, sa, co, so := r.cosAlpha, r.sinAlpha, r.cosOmega, r.sinOmega
	r.visibleFaces = map[string]facePair{}
	for _, part := range memberNames {
		r.visibleFaces[part] = facePair{}
	}
	r.allFaces = append([]string{}, allFaceNames...)

	for _, part := range memberNames {
		var cubeMaxDepth *cubePoint
		r.setCubePoints()
		for index := range r.cubePoints {
			item := &r.cubePoints[index]
			m := r.membersAngles[part]
			item.point.PreProject(0, 0, 0, m.cosAlpha, m.sinAlpha, m.cosOmega, m.sinOmega)
			item.point.Project(ca, sa, co, so, &r.bounds)
			if cubeMaxDepth == nil {
				cubeMaxDepth = item
			} else if cubeMaxDepth.point.Depth(ca, sa, co, so, &r.bounds) >
				item.point.Depth(ca, sa, co, so, &r.bounds) {
				cubeMaxDepth = item
			}
		}
		back := cubeMaxDepth.faces
		r.visibleFaces[part] = facePair{back: back, front: arrayDiff(r.allFaces, back)}
	}

	r.setCubePoints()
	var maxDepth *cubePoint
	for index := range r.cubePoints {
		item := &r.cubePoints[index]
		item.point.Project(ca, sa, co, so, &r.bounds)
		if maxDepth == nil {
			maxDepth = item
		} else if maxDepth.point.Depth(ca, sa, co, so, &r.bounds) >
			item.point.Depth(ca, sa, co, so, &r.bounds) {
			maxDepth = item
		}
		r.backFaces = maxDepth.faces
		r.frontFaces = arrayDiff(r.allFaces, r.backFaces)
	}
}

func arrayDiff(all, remove []string) []string {
	result := make([]string, 0, len(all))
	for _, face := range all {
		if !containsString(remove, face) {
			result = append(result, face)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (r *SkinRenderer) setCubePoints() {
	r.cubePoints = []cubePoint{
		{NewPoint3(0, 0, 0), []string{"back", "right", "top"}},
		{NewPoint3(0, 0, 1), []string{"front", "right", "top"}},
		{NewPoint3(0, 1, 0), []string{"back", "right", "bottom"}},
		{NewPoint3(0, 1, 1), []string{"front", "right", "bottom"}},
		{NewPoint3(1, 0, 0), []string{"back", "left", "top"}},
		{NewPoint3(1, 0, 1), []string{"front", "left", "top"}},
		{NewPoint3(1, 1, 0), []string{"back", "left", "bottom"}},
		{NewPoint3(1, 1, 1), []string{"front", "left", "bottom"}},
	}
}

func (r *SkinRenderer) generatePolygons() {
	r.polygons = map[string]map[string][]*Polygon{}
	for _, part := range pieceNames {
		faces := map[string][]*Polygon{}
		for _, face := range allFaceNames {
			faces[face] = nil
		}
		r.polygons[part] = faces
	}
	r.head()
	r.helmet()
	if !r.headOnly {
		r.torso()
		r.rightArm()
		r.leftArm()
		r.rightLeg()
		r.leftLeg()
	}
}

func key(i, j, k float64) string {
	return itoa(int64(i)) + "|" + itoa(int64(j)) + "|" + itoa(int64(k))
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buffer [24]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buffer[index] = '-'
	}
	return string(buffer[index:])
}

func (r *SkinRenderer) has(i, j, k float64) bool {
	_, ok := r.volumePoints[key(i, j, k)]
	return ok
}

func (r *SkinRenderer) put(i, j, k, x, y, z float64) {
	r.volumePoints[key(i, j, k)] = NewPoint3(x, y, z)
}

func (r *SkinRenderer) pt(i, j, k float64) *Point3 {
	point, ok := r.volumePoints[key(i, j, k)]
	if !ok {
		panic("volume point missing: " + key(i, j, k))
	}
	return point
}

func (r *SkinRenderer) color(x, y int) uint32 {
	return r.skin.colorAtGD(x, y)
}

func (r *SkinRenderer) quad(part, face string, a, b, c, d *Point3, color uint32) {
	r.polygons[part][face] = append(r.polygons[part][face],
		NewPolygon([]*Point3{a, b, c, d}, color))
}

func (r *SkinRenderer) head() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 9*hd; j++ {
			if !r.has(float64(i), float64(j), -2*hd) {
				r.put(float64(i), float64(j), -2*hd, float64(i), float64(j), -2*hd)
			}
			if !r.has(float64(i), float64(j), 6*hd) {
				r.put(float64(i), float64(j), 6*hd, float64(i), float64(j), 6*hd)
			}
		}
	}
	for j := 0; float64(j) < 9*hd; j++ {
		for k := int(-2 * hd); float64(k) < 7*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), 0, float64(j), float64(k))
			}
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(8*hd, float64(j), float64(k), 8*hd, float64(j), float64(k))
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := int(-2 * hd); float64(k) < 7*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i), 0, float64(k))
			}
			if !r.has(float64(i), 8*hd, float64(k)) {
				r.put(float64(i), 8*hd, float64(k), float64(i), 8*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 8*hd; i++ {
		for j := 0; float64(j) < 8*hd; j++ {
			r.quad("head", "back", r.pt(float64(i), float64(j), -2*hd),
				r.pt(float64(i+1), float64(j), -2*hd),
				r.pt(float64(i+1), float64(j+1), -2*hd),
				r.pt(float64(i), float64(j+1), -2*hd),
				r.color(int(32*hd-1)-i, int(8*hd)+j))
			r.quad("head", "front", r.pt(float64(i), float64(j), 6*hd),
				r.pt(float64(i+1), float64(j), 6*hd),
				r.pt(float64(i+1), float64(j+1), 6*hd),
				r.pt(float64(i), float64(j+1), 6*hd),
				r.color(int(8*hd)+i, int(8*hd)+j))
		}
	}
	for j := 0; float64(j) < 8*hd; j++ {
		for k := int(-2 * hd); float64(k) < 6*hd; k++ {
			r.quad("head", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)),
				r.color(k+int(2*hd), int(8*hd)+j))
			r.quad("head", "left", r.pt(8*hd, float64(j), float64(k)),
				r.pt(8*hd, float64(j), float64(k+1)),
				r.pt(8*hd, float64(j+1), float64(k+1)),
				r.pt(8*hd, float64(j+1), float64(k)),
				r.color(int(24*hd)-1-k-int(2*hd), int(8*hd)+j))
		}
	}
	for i := 0; float64(i) < 8*hd; i++ {
		for k := int(-2 * hd); float64(k) < 6*hd; k++ {
			r.quad("head", "top", r.pt(float64(i), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k+1)),
				r.pt(float64(i), 0, float64(k+1)),
				r.color(int(8*hd)+i, k+int(2*hd)))
			r.quad("head", "bottom", r.pt(float64(i), 8*hd, float64(k)),
				r.pt(float64(i+1), 8*hd, float64(k)),
				r.pt(float64(i+1), 8*hd, float64(k+1)),
				r.pt(float64(i), 8*hd, float64(k+1)),
				r.color(int(16*hd)+i, int(2*hd)+k))
		}
	}
}

func (r *SkinRenderer) helmet() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 9*hd; j++ {
			if !r.has(float64(i), float64(j), -2*hd) {
				r.put(float64(i), float64(j), -2*hd,
					float64(i)*9/8.0-0.5*hd, float64(j)*9/8.0-0.5*hd, -2.5*hd)
			}
			if !r.has(float64(i), float64(j), 6*hd) {
				r.put(float64(i), float64(j), 6*hd,
					float64(i)*9/8.0-0.5*hd, float64(j)*9/8.0-0.5*hd, 6.5*hd)
			}
		}
	}
	for j := 0; float64(j) < 9*hd; j++ {
		for k := int(-2 * hd); float64(k) < 7*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), -0.5*hd,
					float64(j)*9/8.0-0.5*hd, float64(k)*9/8.0-0.5*hd)
			}
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(8*hd, float64(j), float64(k), 8.5*hd,
					float64(j)*9/8.0-0.5*hd, float64(k)*9/8.0-0.5*hd)
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := int(-2 * hd); float64(k) < 7*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i)*9/8.0-0.5*hd,
					-0.5*hd, float64(k)*9/8.0-0.5*hd)
			}
			if !r.has(float64(i), 8*hd, float64(k)) {
				r.put(float64(i), 8*hd, float64(k), float64(i)*9/8.0-0.5*hd,
					8.5*hd, float64(k)*9/8.0-0.5*hd)
			}
		}
	}
	for i := 0; float64(i) < 8*hd; i++ {
		for j := 0; float64(j) < 8*hd; j++ {
			r.quad("helmet", "back", r.pt(float64(i), float64(j), -2*hd),
				r.pt(float64(i+1), float64(j), -2*hd),
				r.pt(float64(i+1), float64(j+1), -2*hd),
				r.pt(float64(i), float64(j+1), -2*hd),
				r.color(int(32*hd+32*hd-1)-i, int(8*hd)+j))
			r.quad("helmet", "front", r.pt(float64(i), float64(j), 6*hd),
				r.pt(float64(i+1), float64(j), 6*hd),
				r.pt(float64(i+1), float64(j+1), 6*hd),
				r.pt(float64(i), float64(j+1), 6*hd),
				r.color(int(32*hd+8*hd)+i, int(8*hd)+j))
		}
	}
	for j := 0; float64(j) < 8*hd; j++ {
		for k := int(-2 * hd); float64(k) < 6*hd; k++ {
			r.quad("helmet", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)),
				r.color(int(32*hd)+k+int(2*hd), int(8*hd)+j))
			r.quad("helmet", "left", r.pt(8*hd, float64(j), float64(k)),
				r.pt(8*hd, float64(j), float64(k+1)),
				r.pt(8*hd, float64(j+1), float64(k+1)),
				r.pt(8*hd, float64(j+1), float64(k)),
				r.color(int(32*hd+24*hd-1)-k-int(2*hd), int(8*hd)+j))
		}
	}
	for i := 0; float64(i) < 8*hd; i++ {
		for k := int(-2 * hd); float64(k) < 6*hd; k++ {
			r.quad("helmet", "top", r.pt(float64(i), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k+1)),
				r.pt(float64(i), 0, float64(k+1)),
				r.color(int(32*hd+8*hd)+i, k+int(2*hd)))
			r.quad("helmet", "bottom", r.pt(float64(i), 8*hd, float64(k)),
				r.pt(float64(i+1), 8*hd, float64(k)),
				r.pt(float64(i+1), 8*hd, float64(k+1)),
				r.pt(float64(i), 8*hd, float64(k+1)),
				r.color(int(32*hd+16*hd)+i, int(2*hd)+k))
		}
	}
}

func (r *SkinRenderer) torso() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 13*hd; j++ {
			if !r.has(float64(i), float64(j), 0) {
				r.put(float64(i), float64(j), 0, float64(i), float64(j)+8*hd, 0)
			}
			if !r.has(float64(i), float64(j), 4*hd) {
				r.put(float64(i), float64(j), 4*hd, float64(i), float64(j)+8*hd, 4*hd)
			}
		}
	}
	for j := 0; float64(j) < 13*hd; j++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), 0, float64(j)+8*hd, float64(k))
			}
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(8*hd, float64(j), float64(k), 8*hd, float64(j)+8*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i), 8*hd, float64(k))
			}
			if !r.has(float64(i), 12*hd, float64(k)) {
				r.put(float64(i), 12*hd, float64(k), float64(i), 20*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 8*hd; i++ {
		for j := 0; float64(j) < 12*hd; j++ {
			r.quad("torso", "back", r.pt(float64(i), float64(j), 0),
				r.pt(float64(i+1), float64(j), 0),
				r.pt(float64(i+1), float64(j+1), 0),
				r.pt(float64(i), float64(j+1), 0),
				r.color(int(40*hd-1)-i, int(20*hd)+j))
			r.quad("torso", "front", r.pt(float64(i), float64(j), 4*hd),
				r.pt(float64(i+1), float64(j), 4*hd),
				r.pt(float64(i+1), float64(j+1), 4*hd),
				r.pt(float64(i), float64(j+1), 4*hd),
				r.color(int(20*hd)+i, int(20*hd)+j))
		}
	}
	for j := 0; float64(j) < 12*hd; j++ {
		for k := 0; float64(k) < 4*hd; k++ {
			r.quad("torso", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)),
				r.color(int(16*hd)+k, int(20*hd)+j))
			r.quad("torso", "left", r.pt(8*hd, float64(j), float64(k)),
				r.pt(8*hd, float64(j), float64(k+1)),
				r.pt(8*hd, float64(j+1), float64(k+1)),
				r.pt(8*hd, float64(j+1), float64(k)),
				r.color(int(32*hd-1)-k, int(20*hd)+j))
		}
	}
	for i := 0; float64(i) < 8*hd; i++ {
		for k := 0; float64(k) < 4*hd; k++ {
			r.quad("torso", "top", r.pt(float64(i), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k+1)),
				r.pt(float64(i), 0, float64(k+1)),
				r.color(int(20*hd)+i, int(16*hd)+k))
			r.quad("torso", "bottom", r.pt(float64(i), 12*hd, float64(k)),
				r.pt(float64(i+1), 12*hd, float64(k)),
				r.pt(float64(i+1), 12*hd, float64(k+1)),
				r.pt(float64(i), 12*hd, float64(k+1)),
				r.color(int(28*hd)+i, int(20*hd-1)-k))
		}
	}
}

func (r *SkinRenderer) rightArm() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 13*hd; j++ {
			if !r.has(float64(i), float64(j), 0) {
				r.put(float64(i), float64(j), 0, float64(i)-4*hd, float64(j)+8*hd, 0)
			}
			if !r.has(float64(i), float64(j), 4*hd) {
				r.put(float64(i), float64(j), 4*hd, float64(i)-4*hd,
					float64(j)+8*hd, 4*hd)
			}
		}
	}
	for j := 0; float64(j) < 13*hd; j++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), -4*hd, float64(j)+8*hd, float64(k))
			}
			// 上游原样：守卫检查 8*hd，写入 4*hd。
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(4*hd, float64(j), float64(k), 0, float64(j)+8*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i)-4*hd, 8*hd, float64(k))
			}
			if !r.has(float64(i), 12*hd, float64(k)) {
				r.put(float64(i), 12*hd, float64(k), float64(i)-4*hd, 20*hd, float64(k))
			}
		}
	}
	if r.isAlex {
		for i := 0; float64(i) < 3*hd; i++ {
			for j := 0; float64(j) < 12*hd; j++ {
				r.quad("rightArm", "back", r.pt(float64(i), float64(j), 0),
					r.pt(float64(i+1), float64(j), 0),
					r.pt(float64(i+1), float64(j+1), 0),
					r.pt(float64(i), float64(j+1), 0),
					r.color(int(51*hd-1)-i, int(20*hd)+j))
				r.quad("rightArm", "front", r.pt(float64(i), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j+1), 4*hd),
					r.pt(float64(i), float64(j+1), 4*hd),
					r.color(int(44*hd)+i, int(20*hd)+j))
			}
		}
	} else {
		for i := 0; float64(i) < 4*hd; i++ {
			for j := 0; float64(j) < 12*hd; j++ {
				r.quad("rightArm", "back", r.pt(float64(i), float64(j), 0),
					r.pt(float64(i+1), float64(j), 0),
					r.pt(float64(i+1), float64(j+1), 0),
					r.pt(float64(i), float64(j+1), 0),
					r.color(int(56*hd-1)-i, int(20*hd)+j))
				r.quad("rightArm", "front", r.pt(float64(i), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j+1), 4*hd),
					r.pt(float64(i), float64(j+1), 4*hd),
					r.color(int(44*hd)+i, int(20*hd)+j))
			}
		}
	}
	for j := 0; float64(j) < 12*hd; j++ {
		for k := 0; float64(k) < 4*hd; k++ {
			rightOffsetX := 40.0
			if r.isAlex {
				rightOffsetX = 47
			}
			r.quad("rightArm", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)),
				r.color(int(rightOffsetX*hd)+k, int(20*hd)+j))
			leftOffsetX := 52.0
			if r.isAlex {
				leftOffsetX = 40
			}
			r.quad("rightArm", "left", r.pt(4*hd, float64(j), float64(k)),
				r.pt(4*hd, float64(j), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k)),
				r.color(int(leftOffsetX*hd-1)-k, int(20*hd)+j))
		}
	}
	if r.isAlex {
		for i := 0; float64(i) < 3*hd; i++ {
			for k := 0; float64(k) < 4*hd; k++ {
				r.quad("rightArm", "top", r.pt(float64(i), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k+1)),
					r.pt(float64(i), 0, float64(k+1)),
					r.color(int(44*hd)+i, int(16*hd)+k))
				r.quad("rightArm", "bottom", r.pt(float64(i), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k+1)),
					r.pt(float64(i), 12*hd, float64(k+1)),
					r.color(int(47*hd)+i, int(16*hd)+k))
			}
		}
	} else {
		for i := 0; float64(i) < 4*hd; i++ {
			for k := 0; float64(k) < 4*hd; k++ {
				r.quad("rightArm", "top", r.pt(float64(i), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k+1)),
					r.pt(float64(i), 0, float64(k+1)),
					r.color(int(44*hd)+i, int(16*hd)+k))
				r.quad("rightArm", "bottom", r.pt(float64(i), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k+1)),
					r.pt(float64(i), 12*hd, float64(k+1)),
					r.color(int(48*hd)+i, int(16*hd)+k))
			}
		}
	}
}

func (r *SkinRenderer) leftArm() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 13*hd; j++ {
			if !r.has(float64(i), float64(j), 0) {
				r.put(float64(i), float64(j), 0, float64(i)+8*hd, float64(j)+8*hd, 0)
			}
			if !r.has(float64(i), float64(j), 4*hd) {
				r.put(float64(i), float64(j), 4*hd, float64(i)+8*hd,
					float64(j)+8*hd, 4*hd)
			}
		}
	}
	for j := 0; float64(j) < 13*hd; j++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), 8*hd, float64(j)+8*hd, float64(k))
			}
			// 上游原样：守卫检查 8*hd，写入 4*hd。
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(4*hd, float64(j), float64(k), 12*hd,
					float64(j)+8*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i)+8*hd, 8*hd, float64(k))
			}
			if !r.has(float64(i), 12*hd, float64(k)) {
				r.put(float64(i), 12*hd, float64(k), float64(i)+8*hd, 20*hd, float64(k))
			}
		}
	}
	if r.isAlex {
		for i := 0; float64(i) < 3*hd; i++ {
			for j := 0; float64(j) < 12*hd; j++ {
				r.quad("leftArm", "back", r.pt(float64(i), float64(j), 0),
					r.pt(float64(i+1), float64(j), 0),
					r.pt(float64(i+1), float64(j+1), 0),
					r.pt(float64(i), float64(j+1), 0),
					r.color(int(43*hd)+i, int(52*hd)+j))
				r.quad("leftArm", "front", r.pt(float64(i), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j+1), 4*hd),
					r.pt(float64(i), float64(j+1), 4*hd),
					r.color(int(36*hd)+i, int(52*hd)+j))
			}
		}
	} else {
		for i := 0; float64(i) < 4*hd; i++ {
			for j := 0; float64(j) < 12*hd; j++ {
				var color1, color2 uint32
				if r.isNewSkinType {
					color1 = r.color(int(47*hd)-i, int(52*hd)+j)
					color2 = r.color(int(36*hd)+i, int(52*hd)+j)
				} else {
					color1 = r.color(int(56*hd-1)-(int(4*hd-1)-i), int(20*hd)+j)
					color2 = r.color(int(44*hd)+(int(4*hd-1)-i), int(20*hd)+j)
				}
				r.quad("leftArm", "back", r.pt(float64(i), float64(j), 0),
					r.pt(float64(i+1), float64(j), 0),
					r.pt(float64(i+1), float64(j+1), 0),
					r.pt(float64(i), float64(j+1), 0), color1)
				r.quad("leftArm", "front", r.pt(float64(i), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j), 4*hd),
					r.pt(float64(i+1), float64(j+1), 4*hd),
					r.pt(float64(i), float64(j+1), 4*hd), color2)
			}
		}
	}
	for j := 0; float64(j) < 12*hd; j++ {
		for k := 0; float64(k) < 4*hd; k++ {
			var color1, color2 uint32
			if r.isNewSkinType {
				color1 = r.color(int(32*hd)+k, int(52*hd)+j)
				color2 = r.color(int(43*hd)-k, int(52*hd)+j)
			} else {
				color1 = r.color(int(40*hd)+(int(4*hd-1)-k), int(20*hd)+j)
				color2 = r.color(int(52*hd-1)-(int(4*hd-1)-k), int(20*hd)+j)
			}
			r.quad("leftArm", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)), color1)
			r.quad("leftArm", "left", r.pt(4*hd, float64(j), float64(k)),
				r.pt(4*hd, float64(j), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k)), color2)
		}
	}
	if r.isAlex {
		for i := 0; float64(i) < 3*hd; i++ {
			for k := 0; float64(k) < 4*hd; k++ {
				r.quad("leftArm", "top", r.pt(float64(i), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k+1)),
					r.pt(float64(i), 0, float64(k+1)),
					r.color(int(36*hd)+i, int(48*hd)+k))
				r.quad("leftArm", "bottom", r.pt(float64(i), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k+1)),
					r.pt(float64(i), 12*hd, float64(k+1)),
					r.color(int(39*hd)+i, int(48*hd)+k))
			}
		}
	} else {
		for i := 0; float64(i) < 4*hd; i++ {
			for k := 0; float64(k) < 4*hd; k++ {
				var color1, color2 uint32
				if r.isNewSkinType {
					color1 = r.color(int(36*hd)+i, int(48*hd)+k)
					color2 = r.color(int(40*hd)+i, int(48*hd)+k)
				} else {
					color1 = r.color(int(44*hd)+(int(4*hd-1)-i), int(16*hd)+k)
					color2 = r.color(int(48*hd)+(int(4*hd-1)-i), int(20*hd-1)-k)
				}
				r.quad("leftArm", "top", r.pt(float64(i), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k)),
					r.pt(float64(i+1), 0, float64(k+1)),
					r.pt(float64(i), 0, float64(k+1)), color1)
				r.quad("leftArm", "bottom", r.pt(float64(i), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k)),
					r.pt(float64(i+1), 12*hd, float64(k+1)),
					r.pt(float64(i), 12*hd, float64(k+1)), color2)
			}
		}
	}
}

func (r *SkinRenderer) rightLeg() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 13*hd; j++ {
			if !r.has(float64(i), float64(j), 0) {
				r.put(float64(i), float64(j), 0, float64(i), float64(j)+20*hd, 0)
			}
			if !r.has(float64(i), float64(j), 4*hd) {
				r.put(float64(i), float64(j), 4*hd, float64(i), float64(j)+20*hd, 4*hd)
			}
		}
	}
	for j := 0; float64(j) < 13*hd; j++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), 0, float64(j)+20*hd, float64(k))
			}
			// 上游原样：守卫检查 8*hd，写入 4*hd。
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(4*hd, float64(j), float64(k), 4*hd, float64(j)+20*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i), 20*hd, float64(k))
			}
			if !r.has(float64(i), 12*hd, float64(k)) {
				r.put(float64(i), 12*hd, float64(k), float64(i), 32*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 4*hd; i++ {
		for j := 0; float64(j) < 12*hd; j++ {
			r.quad("rightLeg", "back", r.pt(float64(i), float64(j), 0),
				r.pt(float64(i+1), float64(j), 0),
				r.pt(float64(i+1), float64(j+1), 0),
				r.pt(float64(i), float64(j+1), 0),
				r.color(int(16*hd-1)-i, int(20*hd)+j))
			r.quad("rightLeg", "front", r.pt(float64(i), float64(j), 4*hd),
				r.pt(float64(i+1), float64(j), 4*hd),
				r.pt(float64(i+1), float64(j+1), 4*hd),
				r.pt(float64(i), float64(j+1), 4*hd),
				r.color(int(4*hd)+i, int(20*hd)+j))
		}
	}
	for j := 0; float64(j) < 12*hd; j++ {
		for k := 0; float64(k) < 4*hd; k++ {
			r.quad("rightLeg", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)),
				r.color(k, int(20*hd)+j))
			r.quad("rightLeg", "left", r.pt(4*hd, float64(j), float64(k)),
				r.pt(4*hd, float64(j), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k)),
				r.color(int(12*hd-1)-k, int(20*hd)+j))
		}
	}
	for i := 0; float64(i) < 4*hd; i++ {
		for k := 0; float64(k) < 4*hd; k++ {
			r.quad("rightLeg", "top", r.pt(float64(i), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k+1)),
				r.pt(float64(i), 0, float64(k+1)),
				r.color(int(4*hd)+i, int(16*hd)+k))
			r.quad("rightLeg", "bottom", r.pt(float64(i), 12*hd, float64(k)),
				r.pt(float64(i+1), 12*hd, float64(k)),
				r.pt(float64(i+1), 12*hd, float64(k+1)),
				r.pt(float64(i), 12*hd, float64(k+1)),
				r.color(int(8*hd)+i, int(16*hd)+k))
		}
	}
}

func (r *SkinRenderer) leftLeg() {
	r.volumePoints = map[string]*Point3{}
	hd := r.hd
	for i := 0; float64(i) < 9*hd; i++ {
		for j := 0; float64(j) < 13*hd; j++ {
			if !r.has(float64(i), float64(j), 0) {
				r.put(float64(i), float64(j), 0, float64(i)+4*hd, float64(j)+20*hd, 0)
			}
			if !r.has(float64(i), float64(j), 4*hd) {
				r.put(float64(i), float64(j), 4*hd, float64(i)+4*hd,
					float64(j)+20*hd, 4*hd)
			}
		}
	}
	for j := 0; float64(j) < 13*hd; j++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(0, float64(j), float64(k)) {
				r.put(0, float64(j), float64(k), 4*hd, float64(j)+20*hd, float64(k))
			}
			// 上游原样：守卫检查 8*hd，写入 4*hd。
			if !r.has(8*hd, float64(j), float64(k)) {
				r.put(4*hd, float64(j), float64(k), 8*hd, float64(j)+20*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 9*hd; i++ {
		for k := 0; float64(k) < 5*hd; k++ {
			if !r.has(float64(i), 0, float64(k)) {
				r.put(float64(i), 0, float64(k), float64(i)+4*hd, 20*hd, float64(k))
			}
			if !r.has(float64(i), 12*hd, float64(k)) {
				r.put(float64(i), 12*hd, float64(k), float64(i)+4*hd, 32*hd, float64(k))
			}
		}
	}
	for i := 0; float64(i) < 4*hd; i++ {
		for j := 0; float64(j) < 12*hd; j++ {
			var color1, color2 uint32
			if r.isNewSkinType {
				color1 = r.color(int(31*hd)-i, int(52*hd)+j)
				color2 = r.color(int(20*hd)+i, int(52*hd)+j)
			} else {
				color1 = r.color(int(16*hd-1)-(int(4*hd-1)-i), int(20*hd)+j)
				color2 = r.color(int(4*hd)+(int(4*hd-1)-i), int(20*hd)+j)
			}
			r.quad("leftLeg", "back", r.pt(float64(i), float64(j), 0),
				r.pt(float64(i+1), float64(j), 0),
				r.pt(float64(i+1), float64(j+1), 0),
				r.pt(float64(i), float64(j+1), 0), color1)
			r.quad("leftLeg", "front", r.pt(float64(i), float64(j), 4*hd),
				r.pt(float64(i+1), float64(j), 4*hd),
				r.pt(float64(i+1), float64(j+1), 4*hd),
				r.pt(float64(i), float64(j+1), 4*hd), color2)
		}
	}
	for j := 0; float64(j) < 12*hd; j++ {
		for k := 0; float64(k) < 4*hd; k++ {
			var color1, color2 uint32
			if r.isNewSkinType {
				color1 = r.color(int(16*hd)+k, int(52*hd)+j)
				color2 = r.color(int(27*hd)-k, int(52*hd)+j)
			} else {
				color1 = r.color(int(4*hd-1)-k, int(20*hd)+j)
				color2 = r.color(int(12*hd-1)-(int(4*hd-1)-k), int(20*hd)+j)
			}
			r.quad("leftLeg", "right", r.pt(0, float64(j), float64(k)),
				r.pt(0, float64(j), float64(k+1)),
				r.pt(0, float64(j+1), float64(k+1)),
				r.pt(0, float64(j+1), float64(k)), color1)
			r.quad("leftLeg", "left", r.pt(4*hd, float64(j), float64(k)),
				r.pt(4*hd, float64(j), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k+1)),
				r.pt(4*hd, float64(j+1), float64(k)), color2)
		}
	}
	for i := 0; float64(i) < 4*hd; i++ {
		for k := 0; float64(k) < 4*hd; k++ {
			var color1, color2 uint32
			if r.isNewSkinType {
				color1 = r.color(int(20*hd)+i, int(48*hd)+k)
				color2 = r.color(int(24*hd)+i, int(48*hd)+k)
			} else {
				color1 = r.color(int(4*hd)+(int(4*hd-1)-i), int(16*hd)+k)
				color2 = r.color(int(8*hd)+(int(4*hd-1)-i), int(20*hd-1)-k)
			}
			r.quad("leftLeg", "top", r.pt(float64(i), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k)),
				r.pt(float64(i+1), 0, float64(k+1)),
				r.pt(float64(i), 0, float64(k+1)), color1)
			r.quad("leftLeg", "bottom", r.pt(float64(i), 12*hd, float64(k)),
				r.pt(float64(i+1), 12*hd, float64(k)),
				r.pt(float64(i+1), 12*hd, float64(k+1)),
				r.pt(float64(i), 12*hd, float64(k+1)), color2)
		}
	}
}

func (r *SkinRenderer) memberRotation() {
	r.rotatePart("head", 4, 8, 2)
	r.rotatePart("helmet", 4, 8, 2)
	if !r.headOnly {
		r.rotatePart("rightArm", -2, 8, 2)
		r.rotatePart("leftArm", 10, 8, 2)
		rl := r.membersAngles["rightLeg"]
		z := 4.0
		if rl.sinAlpha < 0 {
			z = 0
		}
		r.rotatePart("rightLeg", 2, 20, z)
		ll := r.membersAngles["leftLeg"]
		z = 4.0
		if ll.sinAlpha < 0 {
			z = 0
		}
		r.rotatePart("leftLeg", 6, 20, z)
	}
}

func (r *SkinRenderer) rotatePart(part string, dx, dy, dz float64) {
	m := r.membersAngles[part]
	for _, polys := range r.polygons[part] {
		for _, poly := range polys {
			poly.PreProject(dx, dy, dz, m.cosAlpha, m.sinAlpha, m.cosOmega, m.sinOmega)
		}
	}
}

func (r *SkinRenderer) createProjectionPlan() {
	ca, sa, co, so := r.cosAlpha, r.sinAlpha, r.cosOmega, r.sinOmega
	for _, piece := range r.polygons {
		for _, polys := range piece {
			for _, poly := range polys {
				if !poly.Projected {
					poly.Project(ca, sa, co, so, &r.bounds)
				}
			}
		}
	}
}

func (r *SkinRenderer) displayImage() *Canvas {
	minX, minY := r.bounds.MinX, r.bounds.MinY
	width := r.bounds.MaxX - r.bounds.MinX
	height := r.bounds.MaxY - r.bounds.MinY
	ratio2 := r.ratio * 2

	srcWidthD := ratio2*width + 1
	srcHeightD := ratio2*height + 1
	srcWidth := int(srcWidthD)
	srcHeight := int(srcHeightD)
	image := NewCanvas(srcWidth, srcHeight)

	for _, pass := range r.getDisplayOrder() {
		for _, face := range pass.faces {
			for _, poly := range r.polygons[pass.piece][face] {
				poly.Draw(image, minX, minY, ratio2)
			}
		}
	}

	realWidth := int(srcWidthD / 2)
	realHeight := int(srcHeightD / 2)
	dest := NewCanvas(realWidth, realHeight)
	dest.ResampleRegion(image, 0, 0, realWidth, realHeight, 0, 0,
		image.Width, image.Height)
	return dest
}

func (r *SkinRenderer) visibleFront(part string) []string {
	return r.visibleFaces[part].front
}

func (r *SkinRenderer) getDisplayOrder() []drawPass {
	var order []drawPass
	if containsString(r.frontFaces, "top") {
		if containsString(r.frontFaces, "right") {
			order = append(order,
				drawPass{"leftLeg", r.backFaces}, drawPass{"leftLeg", r.visibleFront("leftLeg")},
				drawPass{"rightLeg", r.backFaces}, drawPass{"rightLeg", r.visibleFront("rightLeg")},
				drawPass{"leftArm", r.backFaces}, drawPass{"leftArm", r.visibleFront("leftArm")},
				drawPass{"torso", r.backFaces}, drawPass{"torso", r.visibleFront("torso")},
				drawPass{"rightArm", r.backFaces}, drawPass{"rightArm", r.visibleFront("rightArm")})
		} else {
			order = append(order,
				drawPass{"rightLeg", r.backFaces}, drawPass{"rightLeg", r.visibleFront("rightLeg")},
				drawPass{"leftLeg", r.backFaces}, drawPass{"leftLeg", r.visibleFront("leftLeg")},
				drawPass{"rightArm", r.backFaces}, drawPass{"rightArm", r.visibleFront("rightArm")},
				drawPass{"torso", r.backFaces}, drawPass{"torso", r.visibleFront("torso")},
				drawPass{"leftArm", r.backFaces}, drawPass{"leftArm", r.visibleFront("leftArm")})
		}
		order = append(order,
			drawPass{"helmet", r.backFaces}, drawPass{"head", r.backFaces},
			drawPass{"head", r.visibleFront("head")},
			drawPass{"helmet", r.visibleFront("head")})
	} else {
		order = append(order,
			drawPass{"helmet", r.backFaces}, drawPass{"head", r.backFaces},
			drawPass{"head", r.visibleFront("head")},
			drawPass{"helmet", r.visibleFront("head")})
		if containsString(r.frontFaces, "right") {
			order = append(order,
				drawPass{"leftArm", r.backFaces}, drawPass{"leftArm", r.visibleFront("leftArm")},
				drawPass{"torso", r.backFaces}, drawPass{"torso", r.visibleFront("torso")},
				drawPass{"rightArm", r.backFaces}, drawPass{"rightArm", r.visibleFront("rightArm")},
				drawPass{"leftLeg", r.backFaces}, drawPass{"leftLeg", r.visibleFront("leftLeg")},
				drawPass{"rightLeg", r.backFaces}, drawPass{"rightLeg", r.visibleFront("rightLeg")})
		} else {
			order = append(order,
				drawPass{"rightArm", r.backFaces}, drawPass{"rightArm", r.visibleFront("rightArm")},
				drawPass{"torso", r.backFaces}, drawPass{"torso", r.visibleFront("torso")},
				drawPass{"leftArm", r.backFaces}, drawPass{"leftArm", r.visibleFront("leftArm")},
				drawPass{"rightLeg", r.backFaces}, drawPass{"rightLeg", r.visibleFront("rightLeg")},
				drawPass{"leftLeg", r.backFaces}, drawPass{"leftLeg", r.visibleFront("leftLeg")})
		}
	}
	return order
}
