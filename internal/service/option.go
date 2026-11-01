package service

import (
	"database/sql"
	"strconv"
	"strings"
	"sync"

	"neoskin/internal/db"
)

// Option 是 options 表的配置服务（带进程内缓存）。
type Option struct {
	DB      *sql.DB
	Dialect *db.Dialect

	mu     sync.RWMutex
	loaded bool
	items  map[string]string
}

func NewOption(conn *sql.DB, dialect *db.Dialect) *Option {
	return &Option{DB: conn, Dialect: dialect, items: map[string]string{}}
}

// Defaults 对应 config/options.php 中与注册/登录相关的默认值。
var Defaults = map[string]string{
	"register_with_player_name": "true",
	"regs_per_ip":               "3",
	"player_name_rule":          "official",
	"player_name_length_min":    "3",
	"player_name_length_max":    "16",
	"user_initial_score":        "1000",
	"recaptcha_sitekey":         "",
	"recaptcha_secretkey":       "",
	"sign_score":                "10,100",
	"sign_after_zero":           "false",
	"sign_gap_time":             "24",
	"score_per_storage":         "true",
	"score_per_player":          "100",
}

func (o *Option) load() {
	if o == nil || o.DB == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.loaded {
		return
	}
	o.loaded = true

	rows, err := o.DB.Query(db.Rebind(o.Dialect,
		`SELECT option_name, option_value FROM options`))
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err == nil {
			o.items[name] = value
		}
	}
}

func (o *Option) Get(key string) string {
	o.load()
	o.mu.RLock()
	value, ok := o.items[key]
	o.mu.RUnlock()
	if ok {
		return value
	}

	if fallback, ok := Defaults[key]; ok {
		return fallback
	}

	return ""
}

func (o *Option) Bool(key string, fallback bool) bool {
	value := strings.ToLower(strings.Trim(o.Get(key), "()"))
	if value == "" {
		return fallback
	}

	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func (o *Option) Int(key string, fallback int) int {
	value, err := strconv.Atoi(o.Get(key))
	if err != nil {
		return fallback
	}

	return value
}

// Set 更新内存并写库（管理员配置用）。
func (o *Option) Set(key, value string) error {
	o.load()
	o.mu.Lock()
	o.items[key] = value
	o.mu.Unlock()

	update := db.Rebind(o.Dialect,
		`UPDATE options SET option_value = ? WHERE option_name = ?`)
	result, err := o.DB.Exec(update, value, key)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		return nil
	}

	_, err = o.DB.Exec(db.Rebind(o.Dialect,
		`INSERT INTO options (option_name, option_value) VALUES (?, ?)`), key, value)
	return err
}
