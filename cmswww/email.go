package main

import (
	"bytes"
	"html/template"

	"github.com/dajohi/goemail"
)

type NewUserEmailTemplateData struct {
	Token string
	Email string
}

const (
	cmsMailName = "Decred Contractor Management"
)

var (
	templateNewUserEmail = template.Must(
		template.New("new_user_email_template").Parse(templateNewUserEmailRaw))
	templateResetPasswordEmail = template.Must(
		template.New("reset_password_email_template").Parse(templateResetPasswordEmailRaw))
	templateUpdateUserKeyEmail = template.Must(
		template.New("update_user_key_email_template").Parse(templateUpdateUserKeyEmailRaw))
	templateUserLockedResetPassword = template.Must(
		template.New("user_locked_reset_password").Parse(templateUserLockedResetPasswordRaw))
)

// ExecuteTemplate executes a template with the given data.
func ExecuteTemplate(tpl *template.Template, tplData interface{}) error {
	var buf bytes.Buffer
	return tpl.Execute(&buf, &tplData)
}

// emailNewUserVerificationLink emails the link with the new user verification token
// if the email server is set up.
func (c *cmswww) emailNewUserVerificationLink(email, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	var buf bytes.Buffer
	tplData := NewUserEmailTemplateData{
		Email: email,
		Token: token,
	}
	err := templateNewUserEmail.Execute(&buf, &tplData)
	if err != nil {
		return err
	}
	from := "noreply@decred.org"
	subject := "Verify Your Email"
	body := buf.String()

	msg := goemail.NewHTMLMessage(from, subject, body)
	msg.AddTo(email)

	msg.SetName(cmsMailName)
	return c.cfg.SMTP.Send(msg)
}

// emailUserLocked notifies the user its account has been locked and
// emails the link with the reset password verification token
// if the email server is set up.
func (c *cmswww) emailUserLocked(email string) error {
	//if c.cfg.SMTP == nil {
	return nil
	//}
	/*
		l, err := url.Parse(c.cfg.WebServerAddress + ResetPasswordGuiRoute)
		if err != nil {
			return err
		}
		q := l.Query()
		q.Set("email", email)
		l.RawQuery = q.Encode()

		var buf bytes.Buffer
		tplData := resetPasswordEmailTemplateData{
			Email: email,
			Link:  l.String(),
		}
		err = templateUserLockedResetPassword.Execute(&buf, &tplData)
		if err != nil {
			return err
		}
		from := "noreply@decred.org"
		subject := "Locked Account - Reset Your Password"
		body := buf.String()

		msg := goemail.NewHTMLMessage(from, subject, body)
		msg.AddTo(email)

		msg.SetName(cmsMailName)
		return c.cfg.SMTP.Send(msg)
	*/
}
