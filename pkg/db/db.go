package db

import (
	"context"
	"fmt"
	"larsa-academy-microservice/pkg/util"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func InitDB() (*mongo.Client, error) {
	dbFullHost := util.GetEnv("DB_FULL_HOST", "")
	dbPass := util.GetEnv("DB_PASSWORD", "")
	dbUser := util.GetEnv("DB_USER", "")
	host := util.GetEnv("DB_HOST", "localhost")
	dbPort := util.GetEnv("DB_PORT", "27017")

	var uri string
	if dbFullHost != "" {
		uri = fmt.Sprintf("mongodb://%v", dbFullHost)
	} else {
		if dbPass != "" {
			uri = fmt.Sprintf(`mongodb://%v:%v@%v:%v`, dbUser, dbPass, host, dbPort)
		} else {
			uri = fmt.Sprintf("mongodb://%v:%v", host, dbPort)
		}

	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	//SetComponentLevel(options.LogComponentCommand, options.LogLevelDebug)
	//logger := log.New(os.Stdout, "mongo: ", log.LstdFlags)

	bsonOpts := &options.BSONOptions{
		DefaultDocumentM: true,
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMonitor(&event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			//logger.Println(evt.Command)
		},
		Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
			//	logger.Println("Succeeded")
			// logger.Println("Database Name -> ", evt.DatabaseName)
			// logger.Println("Command Name -> ", evt.CommandName)
			// logger.Println("Duration -> ", evt.Duration)
		},
		Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
			//logger.Println("Failed")
			// logger.Println("Database Name -> ", evt.DatabaseName)
			// logger.Println("Command Name -> ", evt.CommandName)
			// logger.Println("Duration -> ", evt.Duration)
		},
	}).SetBSONOptions(bsonOpts))

	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}
	fmt.Println("Mongodb successfully connected and pinged.")

	return client, nil

}

type WithTxn struct {
	db *mongo.Client
}

func (w *WithTxn) Exec(ctx context.Context, callback func(ctx mongo.SessionContext) (any, error)) (any, error) {
	sessionCtx, ok := ctx.(mongo.SessionContext)
	if ok {
		fmt.Println("already session context")
		return callback(sessionCtx)
	}

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Majority()
	txnOptions := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	// Starts a session on the client
	session, err := w.db.StartSession()
	if err != nil {
		return nil, err
	}
	// Defers ending the session after the transaction is committed or ended
	defer session.EndSession(ctx)
	return session.WithTransaction(ctx, callback, txnOptions)
}

func NewWithTxn(i *do.Injector) (*WithTxn, error) {
	return &WithTxn{
		db: do.MustInvoke[*mongo.Client](i),
	}, nil
}
