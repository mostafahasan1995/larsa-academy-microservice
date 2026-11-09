package middleware

import (
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
)

func CapabilityCheck(capability string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := make(chan common.Credentials)

			go common.Guard(c, common.ExtractHeaderParams(r), []string{capability})

			credentials := <-c

			var data *types.CapabilityCheck

			if credentials.Err != nil {
				data = &types.CapabilityCheck{
					Capability: capability,
					IsAllowed:  false,
				}
			} else {
				data = &types.CapabilityCheck{
					Capability: capability,
					IsAllowed:  true,
				}
			}

			ctx := util.SetCapabilityCheck(r.Context(), data)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
