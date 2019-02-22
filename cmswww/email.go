package main

import (
	"bytes"
	"text/template"
	"time"

	"github.com/dajohi/goemail"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

type registerEmailTemplateData struct {
	Token string
	Email string
}
type newIdentityEmailTemplateData struct {
	Token     string
	Email     string
	PublicKey string
}
type userLockedResetPasswordEmailTemplateData struct {
	Email string
}
type resetPasswordEmailTemplateData struct {
	Token string
	Email string
}
type updateExtendedPublicKeyEmailTemplateData struct {
	Token string
	Email string
}
type invoiceApprovedEmailTemplateData struct {
	Date  string
	Token string
}
type invoiceRejectedEmailTemplateData struct {
	Date   string
	Token  string
	Reason string
}
type invoicePaidEmailTemplateData struct {
	Date  string
	Token string
	TxID  string
}

const (
	fromAddress = "noreply@decred.org"
	cmsMailName = "Decred Contractor Management"
)

var (
	templateRegisterEmail = template.Must(
		template.New("register_email_template").Parse(templateRegisterEmailRaw))
	templateNewIdentityEmail = template.Must(
		template.New("new_identity_email_template").Parse(templateNewIdentityEmailRaw))
	templateUserLockedResetPasswordEmail = template.Must(
		template.New("user_locked_reset_password_email_template").Parse(templateUserLockedResetPasswordRaw))
	templateResetPasswordEmail = template.Must(
		template.New("reset_password_email_template").Parse(templateResetPasswordEmailRaw))
	templateUpdateExtendedPublicKeyEmail = template.Must(
		template.New("update_extended_public_key_email_template").Parse(templateUpdateExtendedPublicKeyEmailRaw))
	templateInvoiceApprovedEmail = template.Must(
		template.New("invoice_approved_email_template").Parse(templateInvoiceApprovedEmailRaw))
	templateInvoiceRejectedEmail = template.Must(
		template.New("invoice_rejected_email_template").Parse(templateInvoiceRejectedEmailRaw))
	templateInvoicePaidEmail = template.Must(
		template.New("invoice_paid_email_template").Parse(templateInvoicePaidEmailRaw))
)

func createBody(tpl *template.Template, tplData interface{}) (string, error) {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, tplData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getInvoiceDateStr(dbInvoice *database.Invoice) string {
	t := time.Date(int(dbInvoice.Year), time.Month(dbInvoice.Month),
		1, 0, 0, 0, 0, time.UTC)
	return t.Format("Jan 2006")
}

// sendEmail sends an email with the given subject and body, and the caller
// must supply a function which is used to add email addresses to send the
// email to.
func (c *cmswww) sendEmail(
	subject, body string,
	addToAddressesFn func(*goemail.Message) error,
) error {
	msg := goemail.NewMessage(fromAddress, subject, body)
	err := addToAddressesFn(msg)
	if err != nil {
		return err
	}

	msg.SetName(cmsMailName)
	return c.cfg.SMTP.Send(msg)
}

// sendEmailTo sends an email with the given subject and body to a
// single address.
func (c *cmswww) sendEmailTo(subject, body, toAddress string) error {
	return c.sendEmail(subject, body, func(msg *goemail.Message) error {
		msg.AddTo(toAddress)
		return nil
	})
}

// emailRegisterVerificationLink emails the link with the new user verification token
// if the email server is set up.
func (c *cmswww) emailRegisterVerificationLink(email, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	tplData := registerEmailTemplateData{
		Email: email,
		Token: token,
	}

	subject := "You've been invited!"
	body, err := createBody(templateRegisterEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, email)
}

// emailUpdateIdentityVerificationLink emails the link with the verification token
// used for setting a new key pair if the email server is set up.
func (c *cmswww) emailUpdateIdentityVerificationLink(email, publicKey, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	tplData := newIdentityEmailTemplateData{
		Email: email,
		Token: token,
	}

	subject := "Verify Your New Identity"
	body, err := createBody(templateNewIdentityEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, email)
}

// emailUserLocked notifies the user its account has been locked and
// emails the link with the reset password verification token
// if the email server is set up.
func (c *cmswww) emailUserLocked(email string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	tplData := userLockedResetPasswordEmailTemplateData{
		Email: email,
	}

	subject := "Locked Account - Reset Your Password"
	body, err := createBody(templateUserLockedResetPasswordEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, email)
}

func (c *cmswww) emailResetPasswordVerificationLink(email, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	tplData := resetPasswordEmailTemplateData{
		Email: email,
		Token: token,
	}

	subject := "Verify Your Password Reset"
	body, err := createBody(templateResetPasswordEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, email)
}

func (c *cmswww) emailUpdateExtendedPublicKeyVerificationLink(email, token string) error {
	if c.cfg.SMTP == nil {
		return nil
	}

	tplData := updateExtendedPublicKeyEmailTemplateData{
		Email: email,
		Token: token,
	}

	subject := "Update your Extended Public Key"
	body, err := createBody(templateUpdateExtendedPublicKeyEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, email)
}

func (c *cmswww) emailInvoiceApprovedNotification(
	contractor *database.User,
	dbInvoice *database.Invoice,
) error {
	if c.cfg.SMTP == nil {
		return nil
	}
	if contractor.EmailNotifications&
		uint64(v1.NotificationEmailMyInvoiceApproved) == 0 {
		return nil
	}

	tplData := invoiceApprovedEmailTemplateData{
		Token: dbInvoice.Token,
		Date:  getInvoiceDateStr(dbInvoice),
	}

	subject := "Your invoice has been approved"
	body, err := createBody(templateInvoiceApprovedEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, contractor.Email)
}

func (c *cmswww) emailInvoiceRejectedNotification(
	contractor *database.User,
	dbInvoice *database.Invoice,
) error {
	if c.cfg.SMTP == nil {
		return nil
	}
	if contractor.EmailNotifications&
		uint64(v1.NotificationEmailMyInvoiceRejected) == 0 {
		return nil
	}

	tplData := invoiceRejectedEmailTemplateData{
		Token:  dbInvoice.Token,
		Date:   getInvoiceDateStr(dbInvoice),
		Reason: dbInvoice.StatusChangeReason,
	}

	subject := "Your invoice has been rejected"
	body, err := createBody(templateInvoiceRejectedEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, contractor.Email)
}

func (c *cmswww) emailInvoicePaidNotification(
	contractor *database.User,
	dbInvoice *database.Invoice,
	txID string,
) error {
	if c.cfg.SMTP == nil {
		return nil
	}
	if contractor.EmailNotifications&
		uint64(v1.NotificationEmailMyInvoicePaid) == 0 {
		return nil
	}

	tplData := invoicePaidEmailTemplateData{
		Token: dbInvoice.Token,
		Date:  getInvoiceDateStr(dbInvoice),
		TxID:  txID,
	}

	subject := "Your invoice has been paid"
	body, err := createBody(templateInvoicePaidEmail, &tplData)
	if err != nil {
		return err
	}

	return c.sendEmailTo(subject, body, contractor.Email)
}
