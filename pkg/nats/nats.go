package nats

import (
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/util"
	"log"
	"os"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/interval"
	"git.larsa.io/mahdawi/microservices-commons.git/nats"
	natss "git.larsa.io/mahdawi/microservices-commons.git/nats"
	"github.com/google/uuid"
	"github.com/samber/do"
)

var Injector *do.Injector

func InitNats(injector *do.Injector) {
	Injector = injector
	env := getEnv("NATS_JETSTREAM", "false")
	if env == "false" {
		InitNatsNor()
	} else {
		InitNatsJet()
	}

	// Setup listeners after NATS connection is established
	// SetupListeners is handled by the services themselves or called from main
	// We can inject the listeners initialization here if needed, but it creates circular dependency
	// if we import services/content/listener here.
	// Instead, we should call listener.InitListeners(injector) from main.go
}

// SetupListeners initializes all NATS listeners with dependency injection
func SetupListeners() {
	// This function will be called from the main NATS initialization
	// The actual setup will be done by the individual service listeners
	log.Println("NATS listeners setup initiated")
}

func InitNatsJet() {
	url := fmt.Sprintf(`http://%v:%v`, util.GetEnv("NATS_HOST", "localhost"), util.GetEnv("NATS_PORT", "4223"))
	client := nats.ConnectJet("microservices", uuid.NewString(), url)
	if client == nil {
		err := interval.Do(2, func(attempt int) (bool, error) {
			client = nats.ConnectJet("microservices", uuid.NewString(), url)
			fmt.Println("trying to connect nats server", time.Now())
			if client == nil {
				return true, errors.New("could not connect to nats server")
			}
			return true, nil
		})
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Println("connected to nats server")
		}
	} else {
		fmt.Println("connected to nats server")

	}
}
func InitNatsNor() {
	url := fmt.Sprintf(`http://%v:%v`, util.GetEnv("NATS_HOST", "localhost"), util.GetEnv("NATS_PORT", "4223"))
	client := nats.Connect("microservices", uuid.NewString(), url)
	if client == nil {
		err := interval.Do(2, func(attempt int) (bool, error) {
			client = nats.Connect("microservices", uuid.NewString(), url)
			fmt.Println("trying to connect nats server", time.Now())
			if client == nil {
				return true, errors.New("could not connect to nats server")
			}
			return true, nil
		})
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Println("connected to nats server")
		}
	} else {
		fmt.Println("connected to nats server")

	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// test
func Publish(subject string, data interface{}) error {
	env := getEnv("NATS_JETSTREAM", "false")
	if env == "false" {
		natss.Publish(subject, data)
	} else {
		natss.PublishJet(subject, data)
	}
	return nil
}
