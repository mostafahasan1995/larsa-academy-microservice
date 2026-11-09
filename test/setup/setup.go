package setup

import (
	"context"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/gateway"
	gatewayDi "larsa-academy-microservice/pkg/gateway/di"
	"larsa-academy-microservice/pkg/logging"
	compensation "larsa-academy-microservice/pkg/services/compensation/di"
	contact "larsa-academy-microservice/pkg/services/contact/di"
	content "larsa-academy-microservice/pkg/services/content/di"
	exchange "larsa-academy-microservice/pkg/services/exchange"
	exchangeDi "larsa-academy-microservice/pkg/services/exchange/di"
	messaging "larsa-academy-microservice/pkg/services/messaging/di"
	points "larsa-academy-microservice/pkg/services/points/di"
	sales "larsa-academy-microservice/pkg/services/sales/di"
	sorting "larsa-academy-microservice/pkg/services/sorting/di"
	support "larsa-academy-microservice/pkg/services/support/di"
	"larsa-academy-microservice/pkg/services/zoom"
	zoomDi "larsa-academy-microservice/pkg/services/zoom/di"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	"github.com/samber/do"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	// Shared MongoDB container and client for all tests
	sharedMongoContainer testcontainers.Container
	sharedMongoClient    *mongo.Client
	sharedMongoOnce      sync.Once
	sharedMongoConnStr   string
	originalDBHost       string
	originalDBPort       string
	originalDBFullHost   string
	sharedMongoMutex     sync.RWMutex // Protects client access and reconnection
)

// TestDBConfig holds configuration for test database
type TestDBConfig struct {
	Client   *mongo.Client
	Cleanup  func()
	Database string
}

// initSharedMongoDB initializes a shared MongoDB container that will be reused across all tests
// This is called once using sync.Once to ensure thread-safe initialization
func initSharedMongoDB(t *testing.T) {
	ctx := context.Background()

	// Create MongoDB container with replica set (only once)
	mongoContainer, err := mongodb.Run(
		ctx,
		"mongo:7.0",
		mongodb.WithReplicaSet("rs0"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Waiting for connections").WithOccurrence(1).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start MongoDB container: %v", err)
	}

	// Get host and port directly
	host, err := mongoContainer.Host(ctx)
	if err != nil {
		mongoContainer.Terminate(ctx)
		t.Fatalf("Failed to get MongoDB host: %v", err)
	}
	mappedPort, err := mongoContainer.MappedPort(ctx, "27017")
	if err != nil {
		mongoContainer.Terminate(ctx)
		t.Fatalf("Failed to get MongoDB port: %v", err)
	}

	// Build connection string with replica set
	// Note: db.InitDB will prepend "mongodb://", so we just need host:port/?params
	connStr := fmt.Sprintf("%s:%s/?replicaSet=rs0", host, mappedPort.Port())
	sharedMongoConnStr = connStr

	// Set environment variables for db.InitDB to use
	originalDBHost = os.Getenv("DB_HOST")
	originalDBPort = os.Getenv("DB_PORT")
	originalDBFullHost = os.Getenv("DB_FULL_HOST")

	os.Setenv("DB_FULL_HOST", connStr)
	os.Setenv("DB_HOST", "")
	os.Setenv("DB_PORT", "")

	// Wait for replica set to be ready with proper retry logic
	var client *mongo.Client
	maxWait := 30 * time.Second
	retryInterval := 500 * time.Millisecond
	start := time.Now()

	for {
		client, err = db.InitDB()
		if err == nil {
			// Verify we can actually ping the primary
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = client.Ping(pingCtx, readpref.PrimaryPreferred())
			pingCancel()
			if err == nil {
				break
			}
			client.Disconnect(context.Background())
		}

		if time.Since(start) > maxWait {
			os.Setenv("DB_HOST", originalDBHost)
			os.Setenv("DB_PORT", originalDBPort)
			os.Setenv("DB_FULL_HOST", originalDBFullHost)
			mongoContainer.Terminate(ctx)
			t.Fatalf("Failed to connect to MongoDB after %v: %v", maxWait, err)
		}

		time.Sleep(retryInterval)
	}

	// Store shared resources
	sharedMongoContainer = mongoContainer
	sharedMongoClient = client

	// Note: We don't register cleanup in t.Cleanup here because:
	// 1. t.Cleanup runs when that specific test finishes, not when all tests finish
	// 2. If we clean up the shared container/client when the first test finishes,
	//    it will break all other tests that are still running
	// 3. Testcontainers will automatically clean up the container when the process exits
	// 4. The client will be automatically cleaned up when the process exits
	//
	// If you need explicit cleanup (e.g., for CI/CD), you can use testing.M with
	// a cleanup function, or call CleanupSharedMongoDB() manually at the end of your tests.
}

// SetupTestDB creates or reuses a shared MongoDB testcontainer with replica set enabled
// Returns the client, cleanup function, and a unique database name for this test
// All tests share the same MongoDB instance but use unique database names for isolation
// Note: The MongoDB container is shared across all tests. The container will be
// automatically cleaned up by testcontainers when the process exits. For explicit
// cleanup, use CleanupSharedMongoDB() in testing.M.
func SetupTestDB(t *testing.T) *TestDBConfig {
	// Initialize shared MongoDB container (only once, thread-safe)
	sharedMongoOnce.Do(func() {
		initSharedMongoDB(t)
	})

	// Ensure the client is initialized (with read lock for safety)
	sharedMongoMutex.RLock()
	client := sharedMongoClient
	sharedMongoMutex.RUnlock()

	if client == nil {
		t.Fatal("Shared MongoDB client is nil - initialization may have failed")
	}

	// Verify the client is still connected (it might have been disconnected)
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	needsReconnect := false
	if err := client.Ping(pingCtx, readpref.PrimaryPreferred()); err != nil {
		needsReconnect = true
	}
	pingCancel()

	// Reconnect if needed (with write lock to prevent race conditions)
	if needsReconnect {
		sharedMongoMutex.Lock()
		// Double-check after acquiring lock (another goroutine might have reconnected)
		if sharedMongoClient != nil {
			pingCtx2, pingCancel2 := context.WithTimeout(context.Background(), 2*time.Second)
			if err := sharedMongoClient.Ping(pingCtx2, readpref.PrimaryPreferred()); err == nil {
				// Another goroutine already reconnected, use that client
				client = sharedMongoClient
				needsReconnect = false
			}
			pingCancel2()
		}

		if needsReconnect {
			t.Logf("Warning: Shared MongoDB client was disconnected, reconnecting...")
			// Ensure environment variables are set correctly before reconnecting
			if sharedMongoConnStr != "" {
				os.Setenv("DB_FULL_HOST", sharedMongoConnStr)
				os.Setenv("DB_HOST", "")
				os.Setenv("DB_PORT", "")
			}
			newClient, err := db.InitDB()
			if err != nil {
				sharedMongoMutex.Unlock()
				t.Fatalf("Failed to reconnect to MongoDB: %v", err)
			}
			sharedMongoClient = newClient
			client = newClient
		}
		sharedMongoMutex.Unlock()
	}

	// Note: Environment variables are already set during initialization and shared
	// across all tests. We don't need to set them again here since they're
	// process-wide and all tests use the same MongoDB instance.

	// Generate unique database name for this test (includes test name for better isolation)
	// Sanitize test name to be safe for database names
	// MongoDB has a 63 character limit for database names
	testName := strings.ReplaceAll(t.Name(), "/", "_")
	testName = strings.ReplaceAll(testName, " ", "_")
	// Truncate test name if needed to ensure total length is under 63 chars
	// Format: "test_" + testName + "_" + timestamp (19 chars) = max 63
	// So testName can be at most 63 - 5 - 1 - 19 = 38 chars
	maxTestNameLen := 38
	if len(testName) > maxTestNameLen {
		testName = testName[:maxTestNameLen]
	}
	timestamp := time.Now().UnixNano()
	dbName := fmt.Sprintf("test_%s_%d", testName, timestamp)

	// Cleanup function - only drops the test database, not the container
	cleanup := func() {
		sharedMongoMutex.RLock()
		cleanupClient := sharedMongoClient
		sharedMongoMutex.RUnlock()

		if cleanupClient != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = cleanupClient.Database(dbName).Drop(cleanupCtx)
		}
	}

	return &TestDBConfig{
		Client:   client,
		Cleanup:  cleanup,
		Database: dbName,
	}
}

// SetupTestApp creates a test application with mocked dependencies
// This is a minimal setup. For full app setup, use SetupFullTestApp
func SetupTestApp(t *testing.T, dbConfig *TestDBConfig, authServerURL string) (*do.Injector, *mongo.Database) {
	injector := do.New()

	// Provide database client
	do.ProvideValue(injector, dbConfig.Client)

	// Provide WithTxn
	do.Provide(injector, db.NewWithTxn)

	// Set environment variable for user service URL to point to mock server
	originalUsersURL := os.Getenv("USERS_URL")
	os.Setenv("USERS_URL", authServerURL)

	// Cleanup function to restore env
	t.Cleanup(func() {
		os.Setenv("USERS_URL", originalUsersURL)
	})

	// Get database instance
	database := dbConfig.Client.Database(dbConfig.Database)

	return injector, database
}

type MockPaymentGw struct{}

func (m *MockPaymentGw) PayOrder(ctx context.Context, requestData *gateway.OrderRequest) (*gateway.PaymentInitiationResponseDTO, error) {
	return &gateway.PaymentInitiationResponseDTO{
		OrderId:    primitive.NewObjectID(),
		IsRedirect: false,
	}, nil
}

// SetupFullTestAppWithInjector is like SetupFullTestApp but uses a pre-configured injector
// This is useful when you need to provide custom services (like mocks) before other services are registered
func SetupFullTestAppWithInjector(t *testing.T, injector *do.Injector, dbConfig *TestDBConfig, authServerURL string) (*do.Injector, *chi.Mux, *mongo.Database) {
	// Initialize logger (same as api.Start)
	logging.InitLogger()

	// Setup mocks for Redis and NATS (can be easily replaced with real implementations)
	SetupTestMocks(t)

	// Create router
	r := chi.NewRouter()

	// Setup validator (same as api.Start)
	validateInstance := validator.New()
	validateInstance.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Setup middleware (same as api.Start) - request logging disabled for tests
	// r.Use(middleware.Logger) // disabled to reduce noisy per-request logs during tests
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{

		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id", "Content-Disposition"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Set environment variable for user service URL to point to mock server
	originalUsersURL := os.Getenv("USERS_URL")
	os.Setenv("USERS_URL", authServerURL)
	t.Cleanup(func() {
		os.Setenv("USERS_URL", originalUsersURL)
	})

	// Database setup (same as api.Start) - only provide if not already set
	// Try to get client - if it panics, it's not provided yet
	clientProvided := false
	func() {
		defer func() {
			if r := recover(); r == nil {
				clientProvided = true
			}
		}()
		_ = do.MustInvoke[*mongo.Client](injector)
	}()
	if !clientProvided {
		do.ProvideValue(injector, dbConfig.Client)
		do.Provide(injector, db.NewWithTxn)
	}

	// Validator (same as api.Start)
	do.ProvideValue(injector, validateInstance)

	// Phase 1: Register ALL providers first (same order as api.Start)
	// This ensures all dependencies are registered before any resolution
	sorting.Register(injector, r)
	gatewayDi.Register(injector)

	// Override Payment Gateway with mock for tests
	do.Override(injector, func(i *do.Injector) (gateway.PaymentGw, error) {
		return &MockPaymentGw{}, nil
	})

	messaging.Register(injector, r)
	support.Register(injector, r)
	contact.Register(injector, r)
	content.Register(injector, r)
	sales.Register(injector, r)
	compensation.Register(injector, r)
	// Register exchange - only if not already provided (check by trying to invoke)
	// If exchange service is already provided as a value, skip registration
	exchangeProvided := false
	func() {
		defer func() {
			if r := recover(); r == nil {
				exchangeProvided = true
			}
		}()
		_ = do.MustInvoke[exchange.ExchangeService](injector)
	}()
	if !exchangeProvided {
		exchangeDi.Register(injector)
	}
	points.Register(injector, r)

	// Check if ZoomService is already provided (e.g. by a mock)
	zoomProvided := false
	func() {
		defer func() {
			if r := recover(); r == nil {
				zoomProvided = true
			}
		}()
		_ = do.MustInvoke[zoom.ZoomService](injector)
	}()
	if !zoomProvided {
		zoomDi.Register(injector, r)
	}

	// Phase 2: Register handlers (triggers dependency resolution)
	// All providers are now registered, so circular dependencies are safe
	sorting.RegisterHandlers(injector, r)
	messaging.RegisterHandlers(injector, r)
	support.RegisterHandlers(injector, r)
	contact.RegisterHandlers(injector, r)
	content.RegisterHandlers(injector, r)
	sales.RegisterHandlers(injector, r)
	compensation.RegisterHandlers(injector, r)
	points.RegisterHandlers(injector, r)
	zoomDi.RegisterHandlers(injector, r)

	// Get database instance
	database := dbConfig.Client.Database(dbConfig.Database)

	return injector, r, database
}

// SetupFullTestApp creates a complete test application that mirrors api.Start()
// It includes all service registrations, handlers, middleware, and mocks
// Returns injector, router, and database for full integration testing
func SetupFullTestApp(t *testing.T, dbConfig *TestDBConfig, authServerURL string) (*do.Injector, *chi.Mux, *mongo.Database) {
	// Initialize logger (same as api.Start)
	logging.InitLogger()

	// Setup mocks for Redis and NATS (can be easily replaced with real implementations)
	SetupTestMocks(t)

	// Create injector
	injector := do.New()

	// Create router
	r := chi.NewRouter()

	// Setup validator (same as api.Start)
	validateInstance := validator.New()
	validateInstance.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Setup middleware (same as api.Start) - request logging disabled for tests
	// r.Use(middleware.Logger) // disabled to reduce noisy per-request logs during tests
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{

		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id", "Content-Disposition"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Set environment variable for user service URL to point to mock server
	originalUsersURL := os.Getenv("USERS_URL")
	os.Setenv("USERS_URL", authServerURL)
	t.Cleanup(func() {
		os.Setenv("USERS_URL", originalUsersURL)
	})

	// Database setup (same as api.Start)
	do.ProvideValue(injector, dbConfig.Client)
	do.Provide(injector, db.NewWithTxn)

	// Validator (same as api.Start)
	do.ProvideValue(injector, validateInstance)

	// Phase 1: Register ALL providers first (same order as api.Start)
	// This ensures all dependencies are registered before any resolution
	sorting.Register(injector, r)
	gatewayDi.Register(injector)

	// Override Payment Gateway with mock for tests
	do.Override(injector, func(i *do.Injector) (gateway.PaymentGw, error) {
		return &MockPaymentGw{}, nil
	})

	messaging.Register(injector, r)
	support.Register(injector, r)
	contact.Register(injector, r)
	content.Register(injector, r)
	sales.Register(injector, r)
	compensation.Register(injector, r)
	exchangeDi.Register(injector)
	points.Register(injector, r)
	zoomDi.Register(injector, r)

	// Phase 2: Register handlers (triggers dependency resolution)
	// All providers are now registered, so circular dependencies are safe
	sorting.RegisterHandlers(injector, r)
	messaging.RegisterHandlers(injector, r)
	support.RegisterHandlers(injector, r)
	contact.RegisterHandlers(injector, r)
	content.RegisterHandlers(injector, r)
	sales.RegisterHandlers(injector, r)
	compensation.RegisterHandlers(injector, r)
	points.RegisterHandlers(injector, r)
	zoomDi.RegisterHandlers(injector, r)

	// Get database instance
	database := dbConfig.Client.Database(dbConfig.Database)

	return injector, r, database
}

// SetupTestMocks initializes mock implementations for Redis and NATS
// These can be easily replaced with real testcontainers implementations later
// by modifying this function or adding SetupTestRedis/SetupTestNATS functions
//
// To use real Redis later:
//  1. Import: "larsa-academy-microservice/pkg/caching"
//  2. Replace this function body with: caching.InitRedis()
//  3. Or use testcontainers for Redis: redisContainer := redis.RunContainer(...)
//
// To use real NATS later:
//  1. Import: "larsa-academy-microservice/pkg/nats"
//  2. Add: nats.InitNats()
//  3. Or use testcontainers for NATS: natsContainer := nats.RunContainer(...)
func SetupTestMocks(t *testing.T) {
	// Mock Redis - currently not initialized (services should handle nil gracefully)
	// If your services require a cache, uncomment and create a mock:
	//   import "larsa-academy-microservice/pkg/caching"
	//   caching.Rdb = &MockCache{} // or use real Redis with testcontainers

	// Mock NATS - currently commented out in api.Start, so we skip it
	// If needed, uncomment and initialize:
	//   import "larsa-academy-microservice/pkg/nats"
	//   nats.InitNats() // or use testcontainers for NATS
}

// GetTestDatabase returns the test database instance
func GetTestDatabase(dbConfig *TestDBConfig) *mongo.Database {
	return dbConfig.Client.Database(dbConfig.Database)
}

// CleanupSharedMongoDB manually cleans up the shared MongoDB container and client
// This can be called from testing.M or at the end of your test suite
// if you need explicit cleanup before the process exits
func CleanupSharedMongoDB() {
	if sharedMongoContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = sharedMongoContainer.Terminate(ctx)
	}

	if sharedMongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sharedMongoClient.Disconnect(ctx)
	}

	// Restore original env vars
	os.Setenv("DB_HOST", originalDBHost)
	os.Setenv("DB_PORT", originalDBPort)
	os.Setenv("DB_FULL_HOST", originalDBFullHost)
}
