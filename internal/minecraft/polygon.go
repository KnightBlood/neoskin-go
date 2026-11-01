package minecraft

// Polygon 对应 Blessing\Renderer\Polygon：一个贴图面片（GD int 颜色）。
type Polygon struct {
	Dots      []*Point3
	Color     uint32
	Projected bool
}

// NewPolygon 创建一个面片；四点必须共面（同一坐标轴）。
func NewPolygon(dots []*Point3, color uint32) *Polygon {
	if len(dots) >= 3 {
		sameX := dots[0].X == dots[1].X && dots[1].X == dots[2].X
		sameY := dots[0].Y == dots[1].Y && dots[1].Y == dots[2].Y
		sameZ := dots[0].Z == dots[1].Z && dots[1].Z == dots[2].Z
		if !sameX && !sameY && !sameZ {
			panic("polygon is not on a coordinate plane")
		}
	}
	return &Polygon{Dots: dots, Color: color}
}

// Draw 对应 addPngPolygon()：投影后按 ratio 缩放并填充。
func (p *Polygon) Draw(image *Canvas, minX, minY, ratio float64) {
	if alpha255(p.Color) <= 0 || len(p.Dots) == 0 {
		return
	}
	firstX := p.Dots[0].DestX
	firstY := p.Dots[0].DestY
	samePlanX, samePlanY := true, true
	for _, dot := range p.Dots {
		if dot.DestX != firstX {
			samePlanX = false
		}
		if dot.DestY != firstY {
			samePlanY = false
		}
	}
	if samePlanX || samePlanY {
		return
	}
	points := make([]float64, len(p.Dots)*2)
	for i, dot := range p.Dots {
		points[i*2] = (dot.DestX - minX) * ratio
		points[i*2+1] = (dot.DestY - minY) * ratio
	}
	image.FilledPolygon(points, p.Color)
}

// Project 对应 project()。
func (p *Polygon) Project(cosAlpha, sinAlpha, cosOmega, sinOmega float64, bounds *Bounds) {
	for _, dot := range p.Dots {
		if !dot.Projected {
			dot.Project(cosAlpha, sinAlpha, cosOmega, sinOmega, bounds)
		}
	}
	p.Projected = true
}

// PreProject 对应 preProject()。
func (p *Polygon) PreProject(dx, dy, dz, cosAlpha, sinAlpha, cosOmega, sinOmega float64) {
	for _, dot := range p.Dots {
		dot.PreProject(dx, dy, dz, cosAlpha, sinAlpha, cosOmega, sinOmega)
	}
}
