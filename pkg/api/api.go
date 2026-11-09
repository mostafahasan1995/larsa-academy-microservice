package api

import (

	// "context"
	"fmt"
	"larsa-academy-microservice/pkg/caching"
	"larsa-academy-microservice/pkg/db"
	gateway "larsa-academy-microservice/pkg/gateway/di"
	"larsa-academy-microservice/pkg/logger"
	"larsa-academy-microservice/pkg/nats"
	"larsa-academy-microservice/scheduler"

	compensation "larsa-academy-microservice/pkg/services/compensation/di"
	contact "larsa-academy-microservice/pkg/services/contact/di"
	content "larsa-academy-microservice/pkg/services/content/di"
	"larsa-academy-microservice/pkg/services/content/listener"
	exchange "larsa-academy-microservice/pkg/services/exchange/di"
	points "larsa-academy-microservice/pkg/services/points/di"
	sales "larsa-academy-microservice/pkg/services/sales/di"
	support "larsa-academy-microservice/pkg/services/support/di"

	appMiddleware "larsa-academy-microservice/pkg/middleware"
	messaging "larsa-academy-microservice/pkg/services/messaging/di"
	sorting "larsa-academy-microservice/pkg/services/sorting/di"
	zoom "larsa-academy-microservice/pkg/services/zoom/di"
	"larsa-academy-microservice/pkg/util"

	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	"github.com/samber/do"
)

func Start() error {
	// Initialize colored logger with source location
	//logging.InitLogger()

	logger.Init()

	port := util.GetEnv("PORT", "3279")
	//host := util.GetEnv("HOST", "")

	fmt.Println("start server on port:", port)

	injector := do.New()

	conn, err := db.InitDB()
	if err != nil {
		return err
	}

	caching.InitRedis()

	nats.InitNats(injector)

	r := chi.NewRouter()

	validateInstance := validator.New()

	validateInstance.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

		if name == "-" {
			return ""
		}
		return name
	})

	//r.Use(middleware.Logger)
	r.Use(appMiddleware.ZerologRequestLogger)

	r.Use(middleware.Compress(5))
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id", "Content-Disposition"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	//TODO: UNCOMMENT THIS
	// r.Use(common.ClientSubscriptionInstance.CheckSubscription)

	//ORDER MATTERS . DO NOT CHANGE

	//database
	do.ProvideValue(injector, conn)

	do.Provide(injector, db.NewWithTxn)

	// validator
	do.ProvideValue(injector, validateInstance)

	// Phase 1: Register ALL providers first
	// This ensures all dependencies are registered before any resolution

	//
	sorting.Register(injector, r)
	gateway.Register(injector)
	messaging.Register(injector, r)
	support.Register(injector, r)
	contact.Register(injector, r)
	content.Register(injector, r)
	sales.Register(injector, r)
	compensation.Register(injector, r)
	exchange.Register(injector)
	points.Register(injector, r)
	zoom.Register(injector, r)

	// Phase 2: Register handlers (triggers dependency resolution)
	// All providers are now registered, so circular dependencies are safe

	//
	sorting.RegisterHandlers(injector, r)
	messaging.RegisterHandlers(injector, r)
	support.RegisterHandlers(injector, r)
	contact.RegisterHandlers(injector, r)
	content.RegisterHandlers(injector, r)
	sales.RegisterHandlers(injector, r)
	compensation.RegisterHandlers(injector, r)
	points.RegisterHandlers(injector, r)
	zoom.RegisterHandlers(injector, r)

	listener.InitListeners(injector)

	fmt.Println("start server")

	sch, err := scheduler.NewScheduler(injector)
	if err != nil {
		return err
	}
	sch.Start()
	defer sch.Stop()

	if util.GetEnv("ENVIRONMENT", "") == "development" {
		return http.ListenAndServe("127.0.0.1:"+port, r)
	} else {
		return http.ListenAndServe(":"+port, r)
	}

}
