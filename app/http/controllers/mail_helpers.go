package controllers

import (
	"errors"

	"github.com/goravel/framework/contracts/mail"

	"neoskin/app/facades"
)

// simpleMailable 是最小 Mailable，用于验证/找回密码邮件。
type simpleMailable struct {
	subject string
	html    string
}

func (m simpleMailable) Attachments() []string  { return nil }
func (m simpleMailable) Content() *mail.Content { return &mail.Content{Html: m.html} }
func (m simpleMailable) Headers() map[string]string {
	return map[string]string{"Auto-Submitted": "auto-generated"}
}
func (m simpleMailable) Queue() *mail.Queue { return nil }
func (m simpleMailable) Envelope() *mail.Envelope {
	return &mail.Envelope{Subject: m.subject}
}

// sendMail 通过 Goravel Mail 发送 HTML 邮件；未配置 SMTP 时直接返回错误。
func sendMail(to, subject, html string) error {
	if facades.Config().GetString("mail.host") == "" {
		return errors.New("mail transport is not configured")
	}
	return facades.Mail().To([]string{to}).Send(simpleMailable{subject: subject, html: html})
}
