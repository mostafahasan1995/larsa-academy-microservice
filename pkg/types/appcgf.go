package types

import "git.larsa.io/mahdawi/microservices-commons.git/common"

type AppCfg struct {
	Db      string
	Lang    string
	Hp      *common.HeaderParams
	User    *User
	ReqCaps []string
}
