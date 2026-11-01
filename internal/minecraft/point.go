package minecraft

import "math"

// Bounds 是投影过程的共享 min/max（对应 PHP 引用传参）。
type Bounds struct {
	MinX, MaxX, MinY, MaxY float64
}

// Point3 是 3D 投影点。
type Point3 struct {
	X, Y, Z    float64
	DestX      float64
	DestY      float64
	DestZ      float64
	Projected  bool
	preProject bool
}

func NewPoint3(x, y, z float64) *Point3 {
	return &Point3{X: x, Y: y, Z: z}
}

// Project 对应 project()。
func (p *Point3) Project(cosAlpha, sinAlpha, cosOmega, sinOmega float64, bounds *Bounds) {
	yy, zz := p.Y, p.Z
	p.DestX = p.X*cosOmega + zz*sinOmega
	p.DestY = p.X*sinAlpha*sinOmega + yy*cosAlpha - zz*sinAlpha*cosOmega
	p.DestZ = -p.X*cosAlpha*sinOmega + yy*sinAlpha + zz*cosAlpha*cosOmega
	p.Projected = true
	bounds.MinX = math.Min(bounds.MinX, p.DestX)
	bounds.MaxX = math.Max(bounds.MaxX, p.DestX)
	bounds.MinY = math.Min(bounds.MinY, p.DestY)
	bounds.MaxY = math.Max(bounds.MaxY, p.DestY)
}

// PreProject 对应 preProject()：绕局部轴先旋转。
func (p *Point3) PreProject(dx, dy, dz, cosAlpha, sinAlpha, cosOmega, sinOmega float64) {
	if p.preProject {
		return
	}
	lx, ly, lz := p.X-dx, p.Y-dy, p.Z-dz
	p.X = lx*cosOmega + lz*sinOmega + dx
	p.Y = lx*sinAlpha*sinOmega + ly*cosAlpha - lz*sinAlpha*cosOmega + dy
	p.Z = -lx*cosAlpha*sinOmega + ly*sinAlpha + lz*cosAlpha*cosOmega + dz
	p.preProject = true
}

// Depth 对应 getDepth()。
func (p *Point3) Depth(cosAlpha, sinAlpha, cosOmega, sinOmega float64, bounds *Bounds) float64 {
	if !p.Projected {
		p.Project(cosAlpha, sinAlpha, cosOmega, sinOmega, bounds)
	}
	return p.DestZ
}
