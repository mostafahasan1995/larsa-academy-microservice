package template

import (
	"bytes"
	"context"
	"errors"
	"html/template"
)

type NewsletterWelcomeTpl struct {
	Subject string `bson:"subject" json:"subject"`
	Message string `bson:"message" json:"message"`
}

type NewsletterWelcomeTplData struct {
	CompanyName string
	Email       string
}

func (tpl *NewsletterWelcomeTpl) Construct(ctx context.Context, data any) (string, string, error) {
	d, ok := data.(*NewsletterWelcomeTplData)
	if !ok {
		return "", "", errors.New("invalid data type")
	}
	t := template.New("newsletter_welcome")
	t, errParse := t.Parse(tpl.Message)
	if errParse != nil {
		return "", "", errParse
	}
	temp := new(bytes.Buffer)
	if errEx := t.Execute(temp, d); errEx != nil {
		return "", "", errEx
	}
	return temp.String(), tpl.Subject, nil
}
