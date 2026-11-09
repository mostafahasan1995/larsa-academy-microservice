package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"time"
)

type RenewalTpl struct {
	Subject string `bson:"subject" json:"subject"`
	Message string `bson:"message" json:"message"`
}

type RenewalTplData struct {
	MemberName        string
	RenewalDate       time.Time
	RenewalDateString string
	CompanyName       string
	PlatformName      string
	Items             []string
	Link              string
}

func (rt *RenewalTpl) Construct(ctx context.Context, data any) (string, string, error) {
	d, ok := data.(*RenewalTplData)
	if !ok {
		return "", "", errors.New("invalid renewal data type")
	}

	temp := new(bytes.Buffer)
	t := template.New("t")
	t, errParse := t.Parse(rt.Message)

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
