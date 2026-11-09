package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"git.larsa.io/mahdawi/microservices-commons.git/nats"
	natss "github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"github.com/rs/zerolog/log"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const SubjectUpdatePayment = "ecommerce:updatePayment"

type PaymentOrder struct {
	Id          primitive.ObjectID `json:"_id"`
	ProductId   primitive.ObjectID `json:"productId"`
	ProductType string             `json:"productType"`
}

type WebhookMsg struct {
	Headers struct {
		Client       string `json:"xClient"`
		Service      string `json:"xService"`
		ServiceToken string `json:"xServiceToken"`
	} `json:"headers"`
	Data struct {
		Order   PaymentOrder `json:"order"`
		Payment struct {
			Id primitive.ObjectID `json:"_id"`
		} `json:"payment"`
	}
	ReceiptURL string `json:"receiptURL"`
	InvoicePDF string `json:"invoicePDF"`
	Result     string `json:"result"`
}

func InitListeners(injector *do.Injector) {
	env := util.GetEnv("NATS_JETSTREAM", "false")
	if env == "false" {
		InitNatsListeners(injector)
	} else {
		InitNatsJet(injector)
	}

}

func InitNatsListeners(injector *do.Injector) {
	nats.Listen(SubjectUpdatePayment, "academy", "academy", func(m *stan.Msg) {
		m.Ack()
		var msg WebhookMsg
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			fmt.Printf("error unmarshalling message: %+v\n", err)
			return
		}

		switch msg.Data.Order.ProductType {
		case "academyOrder":
			handleOrder(injector, msg)
		}

	})
}

func InitNatsJet(injector *do.Injector) {
	natsJetLinsterError := nats.ListenJet(SubjectUpdatePayment, "academy", "academy", func(m *natss.Msg) {
		m.Ack()
		var msg WebhookMsg
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			fmt.Printf("error unmarshalling message: %+v\n", err)
			return
		}

		log.Info().Msgf("listening to academy msg: %+v", msg)

		switch msg.Data.Order.ProductType {
		case "academyOrder":
			handleOrder(injector, msg)
		}
	})
	if natsJetLinsterError != nil {
		fmt.Printf("error listening to nats jet stream: %+v\n", natsJetLinsterError)
	}
}

func handleOrder(injector *do.Injector, msg WebhookMsg) {

	log.Info().Msgf("handling order: %+v", msg)
	cfg := &types.AppCfg{
		Db:   msg.Headers.Client,
		Lang: "en",
		Hp: &common.HeaderParams{
			Client: msg.Headers.Client,
		},
	}
	ctx := context.WithValue(context.TODO(), util.ReqAppCfg, cfg)

	orderService := do.MustInvoke[content.OrderSvcs](injector)

	if _, err := orderService.HandlePaymentSuccess(ctx, msg.Data.Order.ProductId.Hex(), msg.Data.Payment.Id.Hex()); err != nil {
		fmt.Printf("error handling payment success: %+v\n", err)
	}
}
