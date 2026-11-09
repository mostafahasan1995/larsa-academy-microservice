package gateway

import (
	"context"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
)

type Gateway interface {
	Request(ctx context.Context, service, path, method, serviceToken string, body map[string]any) (resp *http.Response, err error)
}

type gateway struct {
}

func NewGateway(i *do.Injector) (Gateway, error) {
	return &gateway{}, nil
}

func (g *gateway) Request(ctx context.Context, service, path, method, serviceToken string, body map[string]any) (resp *http.Response, err error) {

	cfg, err := util.GetReqAppCfg(ctx)

	if err != nil {

		return nil, err
	}

	if serviceToken != "" {
		cfg.Hp.ServiceToken = serviceToken
	}

	newUserReqOp := &common.RequestParams{
		Service: service,
		Path:    path,
		Method:  method,
		Data:    body,
		Header:  cfg.Hp,
	}

	res, err := common.CallService(newUserReqOp)

	return res, err
}
