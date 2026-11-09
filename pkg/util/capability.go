package util

import (
	"context"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
)

func HasCapability(ctx context.Context, capability string) (bool, error) {
	cfg, err := GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	// Extract header params from context
	hp := cfg.Hp

	// Use common.Guard to check capability
	c := make(chan common.Credentials)
	go common.Guard(c, hp, []string{capability})

	credentials := <-c
	return credentials.Err == nil, nil
}
