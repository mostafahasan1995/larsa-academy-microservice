package models

import "git.larsa.io/mahdawi/microservices-commons.git/common"

type Email struct {
	To      string                 `bson:"to" json:"to"`
	Subject string                 `bson:"subject" json:"subject"`
	Message string                 `bson:"message" json:"message"`
	Headers *common.HeaderParams   `bson:"headers" json:"headers"`
	Others  map[string]interface{} `bson:"others" json:"others"`
}
