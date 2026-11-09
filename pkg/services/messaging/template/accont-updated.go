package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
)

type AccountUpdatedTpl struct {
	Subject string `bson:"subject" json:"subject"`
	Message string `bson:"message" json:"message"`
}

type AccountUpdatedTplData struct {
	MemberName   string
	CompanyName  string
	PlatformName string
	Email        string
	Password     string
	Link         string
}

func (tpl *AccountUpdatedTpl) Construct(ctx context.Context, data any) (string, string, error) {
	d, ok := data.(*AccountUpdatedTplData)
	if !ok {
		return "", "", errors.New("invalid data type")
	}

	temp := new(bytes.Buffer)
	t := template.New("t")
	t, errParse := t.Parse(tpl.Message)

	if errParse != nil {
		return "", "", errParse
	}

	if errEx := t.Execute(temp, d); errEx != nil {
		return "", "", errEx
	}

	message := new(bytes.Buffer)
	t2, errParse := template.ParseFiles("public/emailTemplate.html")
	if errParse != nil {
		return "", "", errParse
	}
	t2.Execute(message, template.HTML(temp.String()))

	return message.String(), fmt.Sprintf("Welcome to %s on %s", d.CompanyName, d.PlatformName), nil
}
