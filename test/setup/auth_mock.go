package setup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockAuthConfig configures the mock auth server behavior
type MockAuthConfig struct {
	// UserID is the user ID to return in responses
	UserID string
	// Email for the user
	Email string
	// FirstName for the user
	FirstName string
	// LastName for the user
	LastName string
	// Roles that the user has (array of role ObjectIDs as strings)
	Roles []string
	// Capabilities that the user has directly
	Capabilities []string
	// RoleCapabilities maps role IDs to their capabilities
	// Key: role ID (string), Value: array of capabilities
	RoleCapabilities map[string][]string
	// ShouldAuthenticate determines if authentication should succeed
	ShouldAuthenticate bool
	// ShouldAuthorize determines if authorization should succeed
	ShouldAuthorize bool
	// AccessToken that will be accepted (if empty, any token is accepted)
	AccessToken string
	// RolesMap stores role name to role ID mapping for role lookup
	RolesMap map[string]primitive.ObjectID
	// CreatedUsers stores created users (email -> user ID)
	CreatedUsers map[string]primitive.ObjectID
}

// DefaultMockAuthConfig returns a default config with an authenticated, authorized user
func DefaultMockAuthConfig() MockAuthConfig {
	teacherRoleID := primitive.NewObjectID()
	studentRoleID := primitive.NewObjectID()

	return MockAuthConfig{
		UserID:             "507f1f77bcf86cd799439011",
		Email:              "test@example.com",
		FirstName:          "Test",
		LastName:           "User",
		Roles:              []string{teacherRoleID.Hex()},
		Capabilities:       []string{"authenticate"},
		RoleCapabilities:   make(map[string][]string),
		ShouldAuthenticate: true,
		ShouldAuthorize:    true,
		AccessToken:        "",
		RolesMap: map[string]primitive.ObjectID{
			"teacher": teacherRoleID,
			"student": studentRoleID,
		},
		CreatedUsers: make(map[string]primitive.ObjectID),
	}
}

// MockAuthServer is a mock HTTP server that simulates the user service
type MockAuthServer struct {
	server *httptest.Server
	config MockAuthConfig
}

// NewMockAuthServer creates a new mock auth server
func NewMockAuthServer(config MockAuthConfig) *MockAuthServer {
	mock := &MockAuthServer{
		config: config,
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))

	return mock
}

// URL returns the base URL of the mock server
func (m *MockAuthServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockAuthServer) Close() {
	m.server.Close()
}

// UpdateConfig updates the mock server configuration and ensures consistency
func (m *MockAuthServer) UpdateConfig(config MockAuthConfig) {
	m.config = config
}

// GetCurrentConfig returns the current mock server configuration
func (m *MockAuthServer) GetCurrentConfig() MockAuthConfig {
	return m.config
}

// handleRequest handles incoming requests to the mock server
func (m *MockAuthServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Handle /guard endpoint
	if r.Method == http.MethodPost && r.URL.Path == "/guard" {
		m.handleGuard(w, r)
		return
	}

	// Handle /verify endpoint (for service tokens)
	if r.Method == http.MethodPost && r.URL.Path == "/verify" {
		m.handleVerify(w, r)
		return
	}

	// Handle /roles endpoint (for role lookup)
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/roles") {
		m.handleRoles(w, r)
		return
	}

	// Handle /users endpoint (for user creation)
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/users") {
		m.handleAddUser(w, r)
		return
	}

	// Handle PATCH /users/{id} endpoint (for user updates)
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/users/") {
		m.handleUpdateUser(w, r)
		return
	}

	// Unknown endpoint
	http.Error(w, "Not Found", http.StatusNotFound)
}

// handleGuard handles the /guard endpoint
func (m *MockAuthServer) handleGuard(w http.ResponseWriter, r *http.Request) {
	// Check access token
	accessToken := r.Header.Get("x-access-token")
	if m.config.AccessToken != "" && accessToken != m.config.AccessToken {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(common.Credentials{
			Authentication: false,
			Authorization:  false,
			Err:            fmt.Errorf("invalid token"),
		})
		return
	}

	// Parse restrictions from request body
	var requestBody struct {
		Restrictions []string `json:"restrictions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(common.Credentials{
			Err: fmt.Errorf("invalid request body"),
		})
		return
	}

	restrictions := requestBody.Restrictions

	// If no restrictions, just authenticate
	if len(restrictions) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(common.Credentials{
			Authentication: true,
			Authorization:  true,
			User:           m.config.buildUser(),
		})
		return
	}

	// If only "authenticate" restriction, just authenticate
	if len(restrictions) == 1 && restrictions[0] == "authenticate" {
		if !m.config.ShouldAuthenticate {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(common.Credentials{
				Authentication: false,
				Authorization:  false,
				Err:            fmt.Errorf("not authenticated"),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(common.Credentials{
			Authentication: true,
			Authorization:  false,
			User:           m.config.buildUser(),
		})
		return
	}

	// Check authentication
	if !m.config.ShouldAuthenticate {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(common.Credentials{
			Authentication: false,
			Authorization:  false,
			Err:            fmt.Errorf("not authenticated"),
		})
		return
	}

	// Check authorization (capabilities)
	capabilitiesOnly := make([]string, 0)
	for _, r := range restrictions {
		if r != "authenticate" {
			capabilitiesOnly = append(capabilitiesOnly, r)
		}
	}

	hasCapability := m.config.checkCapabilities(capabilitiesOnly)

	if !hasCapability || !m.config.ShouldAuthorize {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(common.Credentials{
			Authentication: true,
			Authorization:  false,
			User:           m.config.buildUser(),
		})
		return
	}

	// Success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(common.Credentials{
		Authentication: true,
		Authorization:  true,
		User:           m.config.buildUser(),
	})
}

// handleVerify handles the /verify endpoint for service tokens
func (m *MockAuthServer) handleVerify(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !m.config.ShouldAuthenticate {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(common.Credentials{
			Authentication: false,
			Authorization:  false,
			Err:            fmt.Errorf("not authenticated"),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(common.Credentials{
		Authentication: true,
		Authorization:  true,
		User:           m.config.buildUser(),
	})
}

// buildUser constructs a User object from the config
func (m *MockAuthConfig) buildUser() common.User {
	email := m.Email
	if email == "" {
		email = "test@example.com"
	}

	firstName := m.FirstName
	if firstName == "" {
		firstName = "Test"
	}

	lastName := m.LastName
	if lastName == "" {
		lastName = "User"
	}

	userID := m.UserID
	if userID == "" {
		userID = "507f1f77bcf86cd799439011"
	}

	return common.User{
		Id:                userID,
		Email:             email,
		EmailConfirmation: true,
		FirstName:         firstName,
		LastName:          lastName,
		Roles:             m.Roles,
		Capabilities:      m.Capabilities,
		CreatedAt:         time.Now(),
	}
}

// checkCapabilities checks if the user has the required capabilities
// Capabilities can be on the user directly or on their roles
func (m *MockAuthConfig) checkCapabilities(required []string) bool {
	if len(required) == 0 {
		return true
	}

	// Check direct user capabilities
	userCaps := make(map[string]bool)
	for _, cap := range m.Capabilities {
		userCaps[cap] = true
	}

	// Check role capabilities
	for _, roleID := range m.Roles {
		if roleCaps, ok := m.RoleCapabilities[roleID]; ok {
			for _, cap := range roleCaps {
				userCaps[cap] = true
			}
		}
	}

	// Check if all required capabilities are present
	for _, req := range required {
		if !userCaps[req] {
			return false
		}
	}

	return true
}

// handleRoles handles GET /roles?query={"name":{"$eq":"teacher"}}
func (m *MockAuthServer) handleRoles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse query to extract role name
	// Simple parsing for {"name":{"$eq":"teacher"}}
	var roleName string
	if strings.Contains(query, `"$eq"`) {
		// Extract role name from query
		parts := strings.Split(query, `"$eq":"`)
		if len(parts) > 1 {
			roleName = strings.Split(parts[1], `"`)[0]
		}
	}

	if roleName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Look up role in RolesMap
	roleID, exists := m.config.RolesMap[roleName]
	if !exists {
		// Return empty roles list
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roles": []interface{}{},
		})
		return
	}

	// Return role
	type Role struct {
		Id   primitive.ObjectID `json:"_id"`
		Name string             `json:"name"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles": []Role{
			{
				Id:   roleID,
				Name: roleName,
			},
		},
	})
}

// handleAddUser handles POST /users/
func (m *MockAuthServer) handleAddUser(w http.ResponseWriter, r *http.Request) {
	var userData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	email, ok := userData["email"].(string)
	if !ok || email == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check if user already exists
	if _, exists := m.config.CreatedUsers[email]; exists {
		// Return conflict
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"msg": "email already exists",
		})
		return
	}

	// Create new user ID
	userID := primitive.NewObjectID()
	m.config.CreatedUsers[email] = userID

	// Return created user
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{
			"_id": userID,
		},
	})
}

// handleUpdateUser handles PATCH /users/{id}
func (m *MockAuthServer) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/users/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse update data (we don't need to validate it, just accept it)
	var userData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Update user config if needed (for testing purposes)
	if firstName, ok := userData["firstName"].(string); ok {
		m.config.FirstName = firstName
	}
	if lastName, ok := userData["lastName"].(string); ok {
		m.config.LastName = lastName
	}
	if email, ok := userData["email"].(string); ok {
		m.config.Email = email
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": m.config.buildUser(),
	})
}
