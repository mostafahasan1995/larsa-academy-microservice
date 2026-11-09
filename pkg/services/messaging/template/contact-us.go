package template

import (
	"bytes"
	"context"
	"errors"
	"html/template"
)

type ContactUsTpl struct {
	Subject string `bson:"subject" json:"subject"`
	Message string `bson:"message" json:"message"`
}

type ContactUsTplData struct {
	FullName        string
	EmailAddress    string
	PhoneNumber     string
	HowDidYouFindUs string
	Message         string
	Additional      map[string]any
}

func (tpl *ContactUsTpl) Construct(ctx context.Context, data any) (string, string, error) {
	d, ok := data.(*ContactUsTplData)
	if !ok {
		return "", "", errors.New("invalid data type")
	}

	t := template.New("contactus")
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
