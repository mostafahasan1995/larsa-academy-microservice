package helpers

import (
	"context"
	"fmt"
	"larsa-academy-microservice/test/setup"
	"math/rand"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo"
)

// TestFixture holds common test infrastructure
type TestFixture struct {
	Router     *chi.Mux
	DBName     string
	Ctx        context.Context
	AuthServer *setup.MockAuthServer
	DB         *mongo.Database
}

// RandomString generates a random string of given length
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// RandomInt generates a random integer between min and max
func RandomInt(min, max int) int {
	return rand.Intn(max-min+1) + min
}

// RandomEmail generates a random email address
func RandomEmail() string {
	return fmt.Sprintf("test_%s@example.com", RandomString(8))
}

// RandomPhone generates a random phone number
func RandomPhone() string {
	return fmt.Sprintf("+1%09d", RandomInt(100000000, 999999999))
}

// RandomNationalID generates a random national ID
func RandomNationalID() string {
	return fmt.Sprintf("%09d", RandomInt(100000000, 999999999))
}

