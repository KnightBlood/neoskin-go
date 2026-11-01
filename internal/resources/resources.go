// Package resources 内嵌站点静态资源（目前只有默认头像）。
package resources

import _ "embed"

//go:embed misc/textures/avatar2d.png
var avatar2d []byte

//go:embed misc/textures/avatar3d.png
var avatar3d []byte

//go:embed misc/textures/steve.png
var steve []byte

// AvatarFallback 返回默认头像 PNG（对应 resources/misc/textures/avatar{2d,3d}.png）。
func AvatarFallback(threeD bool) []byte {
	if threeD {
		return avatar3d
	}
	return avatar2d
}

// DefaultSkin 返回默认皮肤 PNG（resources/misc/textures/steve.png）。
func DefaultSkin() []byte {
	return steve
}
