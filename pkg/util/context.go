package util

import (
	"context"
	"errors"

	"larsa-academy-microservice/pkg/types"
	"net/http"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
)

type CtxKey int

const (
	_ CtxKey = iota
	ReqAppCfg
	ReqUser
	ReqCapabilityCheck
)

// add AppCfg sruct to ctx for use in repos- usually in handlers
func AddCtxAppCfg(r *http.Request) (context.Context, *types.AppCfg) {
	hp := common.ExtractHeaderParams(r)
	db := hp.Client
	lang := hp.AcceptLanguage

	user, _ := r.Context().Value(ReqUser).(*types.User)

	cfg := &types.AppCfg{
		Db:   db,
		Lang: lang,
		Hp:   hp,
		User: user,
	}

	return context.WithValue(r.Context(), ReqAppCfg, cfg), cfg
}

func GetReqAppCfg(ctx context.Context) (*types.AppCfg, error) {
	cfg, ok := ctx.Value(ReqAppCfg).(*types.AppCfg)
	if !ok {
		return nil, errors.New("error get app cfg")
	}

	return cfg, nil
}

// add user to ctx- usually in middleware
func SetReqUser(ctx context.Context, user *types.User) context.Context {
	return context.WithValue(ctx, ReqUser, user)
}

func SetCapabilityCheck(ctx context.Context, data *types.CapabilityCheck) context.Context {
	return context.WithValue(ctx, ReqCapabilityCheck, data)
}
