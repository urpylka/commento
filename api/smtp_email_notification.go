package main

import (
	"bytes"
	"fmt"
	ht "html/template"
	"net/smtp"
	"os"
	tt "text/template"
    "crypto/tls"
)

type emailNotificationPlugs struct {
	Origin               string
	Kind                 string
	UnsubscribeSecretHex string
	Domain               string
	Path                 string
	CommentHex           string
	CommenterName        string
	Title                string
	Html                 ht.HTML
}

func smtpEmailNotification(to string, toName string, kind string, domain string, path string, commentHex string, commenterName string, title string, html string, unsubscribeSecretHex string) error {
	h, err := tt.New("header").Parse(`MIME-Version: 1.0
From: Commento <{{.FromAddress}}>
To: {{.ToName}} <{{.ToAddress}}>
Content-Type: text/html; charset=UTF-8
Subject: {{.Subject}}

`)
	var header bytes.Buffer
	h.Execute(&header, &headerPlugs{FromAddress: os.Getenv("SMTP_FROM_ADDRESS"), ToAddress: to, ToName: toName, Subject: "[Commento] " + title})

	t, err := ht.ParseFiles(fmt.Sprintf("%s/templates/email-notification.txt", os.Getenv("STATIC")))
	if err != nil {
		logger.Errorf("cannot parse %s/templates/email-notification.txt: %v", os.Getenv("STATIC"), err)
		return errorMalformedTemplate
	}

	var body bytes.Buffer
	err = t.Execute(&body, &emailNotificationPlugs{
		Origin:               os.Getenv("ORIGIN"),
		Kind:                 kind,
		Domain:               domain,
		Path:                 path,
		CommentHex:           commentHex,
		CommenterName:        commenterName,
		Title:                title,
		Html:                 ht.HTML(html),
		UnsubscribeSecretHex: unsubscribeSecretHex,
	})
	if err != nil {
		logger.Errorf("error generating templated HTML for email notification: %v", err)
		return err
	}

	// TLS config
	tlsconfig := &tls.Config {
		InsecureSkipVerify: true,
		ServerName: os.Getenv("SMTP_HOST"),
	}

	// Here is the key, you need to call tls.Dial instead of smtp.Dial
    // for smtp servers running on 465 that require an ssl connection
    // from the very beginning (no starttls)
    conn, err := tls.Dial("tcp", os.Getenv("SMTP_HOST") + ":" + os.Getenv("SMTP_PORT"), tlsconfig)
    if err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

    c, err := smtp.NewClient(conn, os.Getenv("SMTP_HOST"))
    if err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

    // Auth
    if err = c.Auth(smtpAuth); err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

    // To && From
    if err = c.Mail(os.Getenv("SMTP_FROM_ADDRESS")); err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

	for _, addr := range []string{to} {
		if err = c.Rcpt(addr); err != nil {
			logger.Errorf("cannot send email notification: %v", err)
			return errorCannotSendEmail
		}
	}

    // Data
    w, err := c.Data()
    if err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

    _, err = w.Write(concat(header, body))
    if err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

    err = w.Close()
    if err != nil {
        logger.Errorf("cannot send email notification: %v", err)
		return errorCannotSendEmail
    }

    c.Quit()
	return nil
}
