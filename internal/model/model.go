package model

// User 对应 users 表（认证/权限用到的列）。
type User struct {
	UID          int
	Email        string
	Nickname     string
	Password     string
	Permission   int
	Avatar       int
	Score        int
	Verified     bool
	DarkMode     bool
	Locale       string
	IP           string
	RegisterAt   string
	LastSignAt   string
	Verification string
}

const (
	PermissionNormal = 0
	PermissionBanned = -1
	PermissionAdmin  = 1
	PermissionSuper  = 2
)

// IsAdmin 对应 PHP User::isAdmin()（permission >= ADMIN）。
func (u User) IsAdmin() bool {
	return u.Permission >= PermissionAdmin
}

// Player 对应 players 表。
type Player struct {
	PID          int
	UID          int
	Name         string
	TidSkin      int
	TidCape      int
	LastModified string
}

// PlayerDetail 是玩家 + 所属用户权限（Minecraft JSON 契约用）。
type PlayerDetail struct {
	Player
	Permission int
}

// Texture 对应 textures 表。
type Texture struct {
	Tid      int
	Name     string
	Type     string
	Hash     string
	Size     int
	Uploader int
	Public   bool
	UploadAt string
	Likes    int
}

// TextureModel 对应 PHP Texture::getModelAttribute()。
func (t Texture) TextureModel() string {
	if t.Type == "alex" {
		return "slim"
	}

	return "default"
}
