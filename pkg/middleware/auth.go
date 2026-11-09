package middleware

import (
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Auth(restructions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := make(chan common.Credentials)

			go common.Guard(c, common.ExtractHeaderParams(r), restructions)

			credentials := <-c

			if credentials.Err != nil {
				http.Error(w, "not authenticated", http.StatusForbidden)
				return
			}

			_user := credentials.User

			userId, err := primitive.ObjectIDFromHex(_user.Id)
			if err != nil {
				http.Error(w, "error get user id", http.StatusForbidden)
				return
			}

			user := &types.User{
				Id:       userId,
				UserData: _user,
			}

			ctx := util.SetReqUser(r.Context(), user)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth tries to get user info but continues if no token is present
func OptionalAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := make(chan common.Credentials)

			go common.Guard(c, common.ExtractHeaderParams(r), []string{"authenticate"})

			credentials := <-c

			// If there's an error (no token), continue without user context
			if credentials.Err != nil {
				next.ServeHTTP(w, r)
				return
			}

			_user := credentials.User

			userId, err := primitive.ObjectIDFromHex(_user.Id)
			if err != nil {
				// If user ID is invalid, continue without user context
				next.ServeHTTP(w, r)
				return
			}

			user := &types.User{
				Id:       userId,
				UserData: _user,
			}

			ctx := util.SetReqUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
