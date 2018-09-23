package main

import (
	"bytes"
	"html/template"

	"github.com/dajohi/goemail"
)

type RegisterEmailTemplateData struct {
	Token string
	Email string
}
type NewIdentityEmailTemplateData struct {
	Token     string
	Email     string
	PublicKey string
}
type ResetPasswordEmailTemplateData struct {
	Token string
	Email string
}

const (
	cmsMailName = "Decred Contractor Management"
)

var (
	templateRegisterEmail = template.Must(
		template.New("register_email_template").Parse(templateRegisterEmailRaw))
	templateNewIdentityEmail = template.Must(
		template.New("new_identity_email_template").Parse(templateNewIdentityEmailRaw))
	templateUserLockedResetPassword = template.Must(
		template.New("user_locked_reset_password_email_template").Parse(templateUserLockedResetPasswordRaw))
	templateResetPasswordEmail = template.Must(
		template.New("reset_password_email_template").Parse(templateResetPasswordEmailRaw))
)

// ExecuteTemplate executes a template with the given data.
func ExecuteTemplate(tpl *template.Template, tplData interface{}) error {
	var buf bytes.Buffer
	return tpl.Execute(&buf, &tplData)
}

// emailRegisterVerificationLink emails the link with the new user verification token
// if the email server is set up.
func (c *cmswww) emailRegisterVerificationLink(email, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	var buf bytes.Buffer
	tplData := RegisterEmailTemplateData{
		Email: email,
		Token: token,
	}
	err := templateRegisterEmail.Execute(&buf, &tplData)
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

// emailUpdateIdentityVerificationLink emails the link with the verification token
// used for setting a new key pair if the email server is set up.
func (c *cmswww) emailUpdateIdentityVerificationLink(email, publicKey, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	var buf bytes.Buffer
	tplData := NewIdentityEmailTemplateData{
		Email: email,
		Token: token,
	}
	err := templateNewIdentityEmail.Execute(&buf, &tplData)
	if err != nil {
		return err
	}
	from := "noreply@decred.org"
	subject := "Verify Your New Identity"
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

func (c *cmswww) emailResetPasswordVerificationLink(email, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	var buf bytes.Buffer
	tplData := ResetPasswordEmailTemplateData{
		Email: email,
		Token: token,
	}
	err := templateResetPasswordEmail.Execute(&buf, &tplData)
	if err != nil {
		return err
	}
	from := "noreply@decred.org"
	subject := "Verify Your Password Reset"
	body := buf.String()

	msg := goemail.NewHTMLMessage(from, subject, body)
	msg.AddTo(email)

	msg.SetName(cmsMailName)
	return c.cfg.SMTP.Send(msg)
}
