package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/util"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PaymentGw interface {
	PayOrder(ctx context.Context, requestData *OrderRequest) (*PaymentInitiationResponseDTO, error)
}

type paymentGw struct {
}

type OrderRequest struct {
	Provider enums.PaymentGateway `json:"provider"`
	Order    Order                `json:"order"`
}

type Order struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	SuccessURL  *string `json:"successURL,omitempty"`
	CancelURL   *string `json:"cancelURL,omitempty"`
	Callback    string  `json:"callback,omitempty"` // to use with geidea
	ProductType *string `json:"productType"`
	Type        *string `json:"type"`
	//ProductId is the id of the order
	ProductId string       `json:"productId"`
	Email     *string      `json:"email"`
	Username  string       `json:"username"`
	Mobile    *string      `json:"mobile,omitempty"`
	Items     *[]OrderItem `json:"items,omitempty"`
}

type OrderItem struct {
	ProductName string  `json:"productName"`
	UnitPrice   float64 `json:"unitPrice"`
	Quantity    int     `json:"quantity"`
}

type GatewayInternalPaymentResponse struct {
	Result string `json:"result"`
	Flow   string `json:"flow"`
	Data   struct {
		RedirectURL string                 `json:"redirectURL"`
		Order       map[string]interface{} `json:"order"`
		Payment     struct {
			Id primitive.ObjectID `json:"id"`
		} `json:"payment"`
	} `json:"data"`
}

type PaymentInitiationResponseDTO struct {
	OrderId     primitive.ObjectID `json:"orderId"`
	IsRedirect  bool               `json:"isRedirect"`
	RedirectURL *string            `json:"redirectURL"`
}

func NewPaymentGw(i *do.Injector) (PaymentGw, error) {
	return &paymentGw{}, nil
}

func (pgw *paymentGw) PayOrder(ctx context.Context, requestData *OrderRequest) (*PaymentInitiationResponseDTO, error) {
	cfg, _ := util.GetReqAppCfg(ctx)

	// Basic validation (can be expanded)
	if requestData.Order.Email != nil && *requestData.Order.Email == "" {
		// Just a basic check, real validation should be more robust if needed
	}

	var successURL, cancelURL *string

	frontendUrl, err := common.GetOptionValue("FRONTEND_URL", cfg.Hp)
	if err != nil {
		// Fallback or error, for now let's log and maybe fail if critical
		// or assume a default if env vars are used elsewhere.
		// Common practice: if option not found, maybe env var?
		// For now, let's proceed and see if we can get it from somewhere else or fail.
		// The reference code fails, so we will fail too.
		return nil, errors.New("frontend url not found in options")
	}

	if url, ok := frontendUrl.(string); ok {
		s := url + "/cart"
		c := url + "/cancel"
		successURL = &s
		cancelURL = &c
	}

	productType := "academyOrder"
	orderType := "regular"
	apiRequest := &OrderRequest{
		Provider: requestData.Provider,
		Order: Order{
			Amount:      requestData.Order.Amount,
			Currency:    requestData.Order.Currency,
			SuccessURL:  successURL,
			CancelURL:   cancelURL,
			ProductType: &productType,
			Type:        &orderType,
			ProductId:   requestData.Order.ProductId,
			Email:       requestData.Order.Email,
			Username:    requestData.Order.Username,
			Mobile:      requestData.Order.Mobile,
			Items:       requestData.Order.Items,
		},
	}

	if requestData.Provider == enums.MYFATOORAH {
		callbackURL, err := common.GetOptionValue("MYFATOORAH_CALLBACK_URL", cfg.Hp)
		if err == nil {
			if url, ok := callbackURL.(string); ok {
				apiRequest.Order.Callback = url
			}
		} else {
			fmt.Println("MYFATOORA callbackURL not found in options", err)
		}
	}

	var orderRequestMap map[string]interface{}
	jsonData, err := json.Marshal(apiRequest)

	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonData, &orderRequestMap)
	if err != nil {
		return nil, err
	}
	// fmt.Println("orderRequestMap", string(jsonData))

	reqOptions := &common.RequestParams{
		Service: "payment",
		Path:    "payments",
		Method:  "POST",
		Data:    orderRequestMap,
		Header:  cfg.Hp,
	}

	pRes, pErr := common.CallService(reqOptions)
	if pErr != nil {
		fmt.Printf("Failed to call payment service: %v", pErr)
		return nil, pErr
	}
	defer pRes.Body.Close()
	bodyBytes, readErr := io.ReadAll(pRes.Body)
	if readErr != nil {
		return nil, readErr
	}

	// fmt.Println("payment request response", string(bodyBytes))
	var gatewayPaymentResp GatewayInternalPaymentResponse
	if err := json.Unmarshal(bodyBytes, &gatewayPaymentResp); err != nil {
		return nil, err
	}

	if gatewayPaymentResp.Result != "success" {
		return nil, errors.New("payment failed: " + string(bodyBytes))
	}

	// Assuming the payment service returns the ID of the payment transaction
	// We might not need to parse the ObjectID if we treat it as string in some contexts,
	// but the reference uses ObjectID.
	response := &PaymentInitiationResponseDTO{
		OrderId:     gatewayPaymentResp.Data.Payment.Id,
		IsRedirect:  gatewayPaymentResp.Flow == "redirect",
		RedirectURL: &gatewayPaymentResp.Data.RedirectURL,
	}

	return response, nil
}
