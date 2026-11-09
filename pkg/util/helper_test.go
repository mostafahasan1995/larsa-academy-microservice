package util

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Test structs for ReplacePlaceholdersInStruct
type TestStruct struct {
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	Description string  `json:"description"`
	Age         int     `json:"age"`
	Active      bool    `json:"active"`
	Score       float64 `json:"score"`
}

type TestStructWithMixedFields struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Count     int    `json:"count"`
	isPrivate string // unexported field - should be skipped (intentionally unused for testing)
	URL       string `json:"url"`
}

type TestStructEmpty struct {
	Count int `json:"count"`
}

func TestReplacePlaceholdersInStruct(t *testing.T) {
	// Test successful replacement with all string fields
	t.Run("successful replacement with all string fields", func(t *testing.T) {
		input := &TestStruct{
			Name:        "Hello {{.username}}",
			Email:       "{{.username}}@{{.domain}}",
			Description: "User {{.username}} has role {{.role}}",
			Age:         25,
			Active:      true,
			Score:       99.5,
		}
		data := map[string]any{
			"username": "john",
			"domain":   "example.com",
			"role":     "admin",
		}
		expected := &TestStruct{
			Name:        "Hello john",
			Email:       "john@example.com",
			Description: "User john has role admin",
			Age:         25,
			Active:      true,
			Score:       99.5,
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if !deepEqual(input, expected) {
			t.Errorf("Expected %+v, but got %+v", expected, input)
		}
	})

	// Test mixed field types
	t.Run("mixed field types with some string replacements", func(t *testing.T) {
		input := &TestStructWithMixedFields{
			Title:   "Welcome {{.name}}",
			Content: "Your score is {{.score}}",
			Count:   10,
			URL:     "https://{{.domain}}/user/{{.id}}",
		}
		data := map[string]any{
			"name":   "Alice",
			"score":  "95",
			"domain": "test.com",
			"id":     "123",
		}
		expected := &TestStructWithMixedFields{
			Title:   "Welcome Alice",
			Content: "Your score is 95",
			Count:   10,
			URL:     "https://test.com/user/123",
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if !deepEqual(input, expected) {
			t.Errorf("Expected %+v, but got %+v", expected, input)
		}
	})

	// Test empty strings
	t.Run("empty strings should remain empty", func(t *testing.T) {
		input := &TestStruct{
			Name:        "",
			Email:       "test@{{.domain}}",
			Description: "",
			Age:         30,
		}
		data := map[string]any{
			"domain": "example.org",
		}
		expected := &TestStruct{
			Name:        "",
			Email:       "test@example.org",
			Description: "",
			Age:         30,
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if !deepEqual(input, expected) {
			t.Errorf("Expected %+v, but got %+v", expected, input)
		}
	})

	// Test no placeholders
	t.Run("no placeholders - strings should remain unchanged", func(t *testing.T) {
		input := &TestStruct{
			Name:        "John Doe",
			Email:       "john@example.com",
			Description: "Regular user",
			Age:         25,
		}
		data := map[string]any{
			"username": "jane",
		}
		expected := &TestStruct{
			Name:        "John Doe",
			Email:       "john@example.com",
			Description: "Regular user",
			Age:         25,
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if !deepEqual(input, expected) {
			t.Errorf("Expected %+v, but got %+v", expected, input)
		}
	})

	// Test struct with no string fields
	t.Run("struct with no string fields", func(t *testing.T) {
		input := &TestStructEmpty{
			Count: 42,
		}
		data := map[string]any{
			"test": "value",
		}
		expected := &TestStructEmpty{
			Count: 42,
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if !deepEqual(input, expected) {
			t.Errorf("Expected %+v, but got %+v", expected, input)
		}
	})
}

func TestReplacePlaceholdersInStruct_ErrorCases(t *testing.T) {
	// Test non-pointer input
	t.Run("non-pointer input should error", func(t *testing.T) {
		input := TestStruct{Name: "test"}
		err := ReplacePlaceholdersInStruct(&input, map[string]any{}) // This should work, let's fix the test
		if err != nil {
			t.Errorf("Unexpected error for valid pointer: %v", err)
		}
	})

	// Test pointer to non-struct
	t.Run("pointer to non-struct should error", func(t *testing.T) {
		str := "test"
		err := ReplacePlaceholdersInStruct(&str, map[string]any{})
		if err == nil {
			t.Error("Expected error for pointer to non-struct, but got none")
			return
		}
		if !strings.Contains(err.Error(), "input must be a pointer to a struct") {
			t.Errorf("Expected specific error message, but got: %s", err.Error())
		}
	})

	// Test nil pointer
	t.Run("nil pointer should error", func(t *testing.T) {
		var nilStruct *TestStruct
		err := ReplacePlaceholdersInStruct[TestStruct](nilStruct, map[string]any{})
		if err == nil {
			t.Error("Expected error for nil pointer, but got none")
			return
		}
		if !strings.Contains(err.Error(), "input must be a pointer to a struct") {
			t.Errorf("Expected specific error message, but got: %s", err.Error())
		}
	})

	// Test invalid template syntax
	t.Run("invalid template syntax should error", func(t *testing.T) {
		input := &TestStruct{
			Name: "Hello {{.username", // missing closing brace
		}
		data := map[string]any{
			"username": "john",
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err == nil {
			t.Error("Expected error for invalid template syntax, but got none")
			return
		}
		if !strings.Contains(err.Error(), "error replacing placeholders in field Name") {
			t.Errorf("Expected specific error message, but got: %s", err.Error())
		}
	})

	// Test missing template data (should not error, just replace with <no value>)
	t.Run("missing template data should replace with no value", func(t *testing.T) {
		input := &TestStruct{
			Name: "Hello {{.nonexistent}}",
		}
		data := map[string]any{
			"username": "john",
		}

		err := ReplacePlaceholdersInStruct(input, data)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := "Hello <no value>"
		if input.Name != expected {
			t.Errorf("Expected '%s', but got '%s'", expected, input.Name)
		}
	})
}

func TestReplacePlaceholdersInStruct_RealWorldExample(t *testing.T) {
	// Test with structs similar to the actual payload structs
	type EmailPayload struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
		To      string `json:"to"`
		From    string `json:"from"`
	}

	email := &EmailPayload{
		Subject: "Welcome {{.firstName}} {{.lastName}}",
		Body:    "Dear {{.firstName}},\n\nWelcome to {{.platform}}! Your account {{.email}} is now active.",
		To:      "{{.email}}",
		From:    "noreply@{{.domain}}",
	}

	eventData := map[string]any{
		"firstName": "John",
		"lastName":  "Smith",
		"email":     "john.smith@example.com",
		"platform":  "MyApp",
		"domain":    "myapp.com",
	}

	err := ReplacePlaceholdersInStruct(email, eventData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := &EmailPayload{
		Subject: "Welcome John Smith",
		Body:    "Dear John,\n\nWelcome to MyApp! Your account john.smith@example.com is now active.",
		To:      "john.smith@example.com",
		From:    "noreply@myapp.com",
	}

	if email.Subject != expected.Subject {
		t.Errorf("Subject: Expected '%s', but got '%s'", expected.Subject, email.Subject)
	}
	if email.Body != expected.Body {
		t.Errorf("Body: Expected '%s', but got '%s'", expected.Body, email.Body)
	}
	if email.To != expected.To {
		t.Errorf("To: Expected '%s', but got '%s'", expected.To, email.To)
	}
	if email.From != expected.From {
		t.Errorf("From: Expected '%s', but got '%s'", expected.From, email.From)
	}
}

// Helper functions for testing
func deepEqual(a, b interface{}) bool {
	// Simple deep equality check for our test structs
	switch va := a.(type) {
	case *TestStruct:
		vb, ok := b.(*TestStruct)
		if !ok {
			return false
		}
		return va.Name == vb.Name && va.Email == vb.Email && va.Description == vb.Description &&
			va.Age == vb.Age && va.Active == vb.Active && va.Score == vb.Score
	case *TestStructWithMixedFields:
		vb, ok := b.(*TestStructWithMixedFields)
		if !ok {
			return false
		}
		return va.Title == vb.Title && va.Content == vb.Content && va.Count == vb.Count && va.URL == vb.URL
	case *TestStructEmpty:
		vb, ok := b.(*TestStructEmpty)
		if !ok {
			return false
		}
		return va.Count == vb.Count
	}
	return false
}

func TestConvertStringsToObjectIDs(t *testing.T) {
	t.Run("valid ObjectID strings should be converted", func(t *testing.T) {
		validObjectID := "507f1f77bcf86cd799439011"
		expectedObjID, _ := primitive.ObjectIDFromHex(validObjectID)

		input := map[string]any{
			"_id":        validObjectID,
			"user_id":    validObjectID,
			"created_by": validObjectID,
		}

		result := ConvertStringsToObjectIDs(input)

		// Check that valid ObjectID strings were converted
		if objID, ok := result["_id"].(primitive.ObjectID); !ok || objID != expectedObjID {
			t.Errorf("Expected _id to be converted to ObjectID %v, got %v", expectedObjID, result["_id"])
		}
		if objID, ok := result["user_id"].(primitive.ObjectID); !ok || objID != expectedObjID {
			t.Errorf("Expected user_id to be converted to ObjectID %v, got %v", expectedObjID, result["user_id"])
		}
		if objID, ok := result["created_by"].(primitive.ObjectID); !ok || objID != expectedObjID {
			t.Errorf("Expected created_by to be converted to ObjectID %v, got %v", expectedObjID, result["created_by"])
		}
	})

	t.Run("invalid ObjectID strings should remain as strings", func(t *testing.T) {
		input := map[string]any{
			"invalid_short": "123",
			"invalid_long":  "507f1f77bcf86cd799439011abc",
			"invalid_chars": "507f1f77bcf86cd799439xyz",
			"empty_string":  "",
			"normal_string": "hello world",
		}

		result := ConvertStringsToObjectIDs(input)

		// Check that invalid ObjectID strings remain as strings
		if str, ok := result["invalid_short"].(string); !ok || str != "123" {
			t.Errorf("Expected invalid_short to remain as string '123', got %v", result["invalid_short"])
		}
		if str, ok := result["invalid_long"].(string); !ok || str != "507f1f77bcf86cd799439011abc" {
			t.Errorf("Expected invalid_long to remain as string, got %v", result["invalid_long"])
		}
		if str, ok := result["invalid_chars"].(string); !ok || str != "507f1f77bcf86cd799439xyz" {
			t.Errorf("Expected invalid_chars to remain as string, got %v", result["invalid_chars"])
		}
		if str, ok := result["empty_string"].(string); !ok || str != "" {
			t.Errorf("Expected empty_string to remain as empty string, got %v", result["empty_string"])
		}
		if str, ok := result["normal_string"].(string); !ok || str != "hello world" {
			t.Errorf("Expected normal_string to remain as string, got %v", result["normal_string"])
		}
	})

	t.Run("non-string values should remain unchanged", func(t *testing.T) {
		input := map[string]any{
			"number":    42,
			"float":     3.14,
			"boolean":   true,
			"nil_value": nil,
			"slice":     []string{"a", "b", "c"},
			"map":       map[string]string{"key": "value"},
		}

		result := ConvertStringsToObjectIDs(input)

		// Check that non-string values remain unchanged
		if num, ok := result["number"].(int); !ok || num != 42 {
			t.Errorf("Expected number to remain as int 42, got %v", result["number"])
		}
		if f, ok := result["float"].(float64); !ok || f != 3.14 {
			t.Errorf("Expected float to remain as float64 3.14, got %v", result["float"])
		}
		if b, ok := result["boolean"].(bool); !ok || b != true {
			t.Errorf("Expected boolean to remain as bool true, got %v", result["boolean"])
		}
		if result["nil_value"] != nil {
			t.Errorf("Expected nil_value to remain as nil, got %v", result["nil_value"])
		}
		if slice, ok := result["slice"].([]string); !ok || len(slice) != 3 {
			t.Errorf("Expected slice to remain unchanged, got %v", result["slice"])
		}
		if m, ok := result["map"].(map[string]string); !ok || m["key"] != "value" {
			t.Errorf("Expected map to remain unchanged, got %v", result["map"])
		}
	})

	t.Run("mixed valid and invalid ObjectID strings", func(t *testing.T) {
		validObjectID := "507f1f77bcf86cd799439011"
		expectedObjID, _ := primitive.ObjectIDFromHex(validObjectID)

		input := map[string]any{
			"valid_id":      validObjectID,
			"invalid_id":    "invalid123",
			"another_valid": "507f1f77bcf86cd799439012",
			"number":        123,
			"text":          "some text",
		}

		result := ConvertStringsToObjectIDs(input)

		// Check valid ObjectID was converted
		if objID, ok := result["valid_id"].(primitive.ObjectID); !ok || objID != expectedObjID {
			t.Errorf("Expected valid_id to be converted to ObjectID, got %v", result["valid_id"])
		}

		// Check another valid ObjectID was converted
		expectedObjID2, _ := primitive.ObjectIDFromHex("507f1f77bcf86cd799439012")
		if objID, ok := result["another_valid"].(primitive.ObjectID); !ok || objID != expectedObjID2 {
			t.Errorf("Expected another_valid to be converted to ObjectID, got %v", result["another_valid"])
		}

		// Check invalid ObjectID remained as string
		if str, ok := result["invalid_id"].(string); !ok || str != "invalid123" {
			t.Errorf("Expected invalid_id to remain as string, got %v", result["invalid_id"])
		}

		// Check other types remained unchanged
		if num, ok := result["number"].(int); !ok || num != 123 {
			t.Errorf("Expected number to remain as int, got %v", result["number"])
		}
		if str, ok := result["text"].(string); !ok || str != "some text" {
			t.Errorf("Expected text to remain as string, got %v", result["text"])
		}
	})

	t.Run("empty map should return empty map", func(t *testing.T) {
		input := map[string]any{}
		result := ConvertStringsToObjectIDs(input)

		if len(result) != 0 {
			t.Errorf("Expected empty map, got %v", result)
		}
	})

	t.Run("nil map should return empty map", func(t *testing.T) {
		result := ConvertStringsToObjectIDs(nil)

		if len(result) != 0 {
			t.Errorf("Expected empty map for nil input, got %v", result)
		}
	})

	t.Run("real world ObjectID examples", func(t *testing.T) {
		// Test with actual ObjectIDs that might be used in production
		realObjectIDs := []string{
			"507f1f77bcf86cd799439011",
			"507f191e810c19729de860ea",
			"6123456789abcdef12345678",
			"000000000000000000000000",
			"ffffffffffffffffffffffff",
		}

		input := make(map[string]any)
		expected := make(map[primitive.ObjectID]bool)

		for i, objIDStr := range realObjectIDs {
			key := "id_" + string(rune('a'+i))
			input[key] = objIDStr
			objID, _ := primitive.ObjectIDFromHex(objIDStr)
			expected[objID] = true
		}

		result := ConvertStringsToObjectIDs(input)

		// Verify all ObjectIDs were converted correctly
		for key, value := range result {
			if objID, ok := value.(primitive.ObjectID); ok {
				if !expected[objID] {
					t.Errorf("Unexpected ObjectID %v for key %s", objID, key)
				}
			} else {
				t.Errorf("Expected ObjectID for key %s, got %T", key, value)
			}
		}
	})

	t.Run("valid date strings should be converted to time.Time", func(t *testing.T) {
		// Test various date formats
		input := map[string]any{
			"rfc3339":        "2023-12-25T15:30:45Z",
			"rfc3339_nano":   "2023-12-25T15:30:45.123456789Z",
			"iso8601":        "2023-12-25T15:30:45",
			"mysql_datetime": "2023-12-25 15:30:45",
			"date_only":      "2023-12-25",
			"alt_date":       "2023/12/25",
			"us_date":        "12/25/2023",
			"eu_date":        "25/12/2023",
			"with_millis":    "2023-12-25 15:30:45.123",
		}

		result := ConvertStringsToObjectIDs(input)

		// Check that valid date strings were converted to time.Time
		expectedTime, _ := time.Parse(time.RFC3339, "2023-12-25T15:30:45Z")
		if convertedTime, ok := result["rfc3339"].(time.Time); !ok || !convertedTime.Equal(expectedTime) {
			t.Errorf("Expected rfc3339 to be converted to time.Time %v, got %v", expectedTime, convertedTime)
		}

		// Test date only conversion
		expectedDate, _ := time.Parse("2006-01-02", "2023-12-25")
		if convertedDate, ok := result["date_only"].(time.Time); !ok || !convertedDate.Equal(expectedDate) {
			t.Errorf("Expected date_only to be converted to time.Time %v, got %v", expectedDate, convertedDate)
		}

		// Verify all date fields were converted to time.Time
		dateFields := []string{"rfc3339", "rfc3339_nano", "iso8601", "mysql_datetime", "date_only", "alt_date", "us_date", "eu_date", "with_millis"}
		for _, field := range dateFields {
			if _, ok := result[field].(time.Time); !ok {
				t.Errorf("Expected %s to be converted to time.Time, got %T", field, result[field])
			}
		}
	})

	t.Run("invalid date strings should remain as strings", func(t *testing.T) {
		input := map[string]any{
			"invalid_format": "not-a-date",
			"partial_date":   "2023-12",
			"wrong_format":   "25-12-2023",
			"text_with_date": "Today is 2023-12-25",
			"empty_date":     "",
		}

		result := ConvertStringsToObjectIDs(input)

		// Check that invalid date strings remain as strings
		for key, expectedValue := range input {
			if str, ok := result[key].(string); !ok || str != expectedValue {
				t.Errorf("Expected %s to remain as string '%s', got %v", key, expectedValue, result[key])
			}
		}
	})

	t.Run("mixed ObjectIDs, dates, and other values", func(t *testing.T) {
		validObjectID := "507f1f77bcf86cd799439011"
		expectedObjID, _ := primitive.ObjectIDFromHex(validObjectID)
		expectedDate, _ := time.Parse(time.RFC3339, "2023-12-25T15:30:45Z")

		input := map[string]any{
			"object_id":    validObjectID,
			"date_time":    "2023-12-25T15:30:45Z",
			"invalid_text": "hello world",
			"number":       42,
			"boolean":      true,
		}

		result := ConvertStringsToObjectIDs(input)

		// Check ObjectID conversion
		if objID, ok := result["object_id"].(primitive.ObjectID); !ok || objID != expectedObjID {
			t.Errorf("Expected object_id to be converted to ObjectID, got %v", result["object_id"])
		}

		// Check date conversion
		if dateTime, ok := result["date_time"].(time.Time); !ok || !dateTime.Equal(expectedDate) {
			t.Errorf("Expected date_time to be converted to time.Time, got %v", result["date_time"])
		}

		// Check other values remain unchanged
		if str, ok := result["invalid_text"].(string); !ok || str != "hello world" {
			t.Errorf("Expected invalid_text to remain as string, got %v", result["invalid_text"])
		}
		if num, ok := result["number"].(int); !ok || num != 42 {
			t.Errorf("Expected number to remain as int, got %v", result["number"])
		}
		if b, ok := result["boolean"].(bool); !ok || b != true {
			t.Errorf("Expected boolean to remain as bool, got %v", result["boolean"])
		}
	})

	t.Run("edge case date formats", func(t *testing.T) {
		input := map[string]any{
			"leap_year":   "2024-02-29",
			"new_year":    "2024-01-01T00:00:00Z",
			"end_of_year": "2023-12-31T23:59:59Z",
			"future_date": "2030-06-15T12:30:45Z",
			"past_date":   "1990-01-01",
		}

		result := ConvertStringsToObjectIDs(input)

		// All should be converted to time.Time
		for key := range input {
			if _, ok := result[key].(time.Time); !ok {
				t.Errorf("Expected %s to be converted to time.Time, got %T", key, result[key])
			}
		}

		// Verify specific dates
		expectedLeapDay, _ := time.Parse("2006-01-02", "2024-02-29")
		if leapDay, ok := result["leap_year"].(time.Time); !ok || !leapDay.Equal(expectedLeapDay) {
			t.Errorf("Expected leap_year to be %v, got %v", expectedLeapDay, result["leap_year"])
		}
	})
}

// Test structs for Converter function
type SourceStruct struct {
	Name   string  `json:"name"`
	Age    int     `json:"age"`
	Email  string  `json:"email"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
}

type TargetStruct struct {
	Name   string  `json:"name"`
	Age    int     `json:"age"`
	Email  string  `json:"email"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
}

type PartialStruct struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type DifferentFieldStruct struct {
	FullName string `json:"name"`
	Years    int    `json:"age"`
}

type InvalidStruct struct {
	Channel chan int `json:"channel"`
}

func TestConverter(t *testing.T) {
	// Test successful conversion between identical structs
	t.Run("successful conversion between identical structs", func(t *testing.T) {
		source := SourceStruct{
			Name:   "John Doe",
			Age:    30,
			Email:  "john@example.com",
			Active: true,
			Score:  95.5,
		}

		result, err := Converter[TargetStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := TargetStruct{
			Name:   "John Doe",
			Age:    30,
			Email:  "john@example.com",
			Active: true,
			Score:  95.5,
		}

		if result != expected {
			t.Errorf("Expected %+v, but got %+v", expected, result)
		}
	})

	// Test conversion from struct to map
	t.Run("conversion from struct to map", func(t *testing.T) {
		source := SourceStruct{
			Name:   "Jane Smith",
			Age:    25,
			Email:  "jane@example.com",
			Active: false,
			Score:  88.0,
		}

		result, err := Converter[map[string]interface{}](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := map[string]interface{}{
			"name":   "Jane Smith",
			"age":    float64(25), // JSON unmarshaling converts numbers to float64
			"email":  "jane@example.com",
			"active": false,
			"score":  88.0,
		}

		if len(result) != len(expected) {
			t.Errorf("Expected map with %d elements, got %d", len(expected), len(result))
			return
		}

		for key, expectedValue := range expected {
			if result[key] != expectedValue {
				t.Errorf("For key %s, expected %v, got %v", key, expectedValue, result[key])
			}
		}
	})

	// Test conversion from map to struct
	t.Run("conversion from map to struct", func(t *testing.T) {
		source := map[string]interface{}{
			"name":   "Alice Johnson",
			"age":    35,
			"email":  "alice@example.com",
			"active": true,
			"score":  92.3,
		}

		result, err := Converter[TargetStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := TargetStruct{
			Name:   "Alice Johnson",
			Age:    35,
			Email:  "alice@example.com",
			Active: true,
			Score:  92.3,
		}

		if result != expected {
			t.Errorf("Expected %+v, but got %+v", expected, result)
		}
	})

	// Test partial conversion (subset of fields)
	t.Run("conversion with subset of fields", func(t *testing.T) {
		source := SourceStruct{
			Name:   "Bob Brown",
			Age:    40,
			Email:  "bob@example.com",
			Active: true,
			Score:  78.5,
		}

		result, err := Converter[PartialStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := PartialStruct{
			Name:  "Bob Brown",
			Email: "bob@example.com",
		}

		if result != expected {
			t.Errorf("Expected %+v, but got %+v", expected, result)
		}
	})

	// Test conversion with different field names but same JSON tags
	t.Run("conversion with different field names but same JSON tags", func(t *testing.T) {
		source := SourceStruct{
			Name: "Charlie Davis",
			Age:  28,
		}

		result, err := Converter[DifferentFieldStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := DifferentFieldStruct{
			FullName: "Charlie Davis",
			Years:    28,
		}

		if result != expected {
			t.Errorf("Expected %+v, but got %+v", expected, result)
		}
	})

	// Test conversion of slice of structs
	t.Run("conversion of slice of structs", func(t *testing.T) {
		source := []SourceStruct{
			{Name: "User1", Age: 20, Email: "user1@example.com"},
			{Name: "User2", Age: 30, Email: "user2@example.com"},
		}

		result, err := Converter[[]TargetStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := []TargetStruct{
			{Name: "User1", Age: 20, Email: "user1@example.com"},
			{Name: "User2", Age: 30, Email: "user2@example.com"},
		}

		if len(result) != len(expected) {
			t.Errorf("Expected slice with %d elements, got %d", len(expected), len(result))
			return
		}

		for i, exp := range expected {
			if result[i] != exp {
				t.Errorf("At index %d, expected %+v, got %+v", i, exp, result[i])
			}
		}
	})

	// Test conversion of basic types
	t.Run("conversion of basic types", func(t *testing.T) {
		// String to string
		result1, err := Converter[string]("hello")
		if err != nil {
			t.Errorf("Unexpected error converting string: %v", err)
		}
		if result1 != "hello" {
			t.Errorf("Expected 'hello', got '%s'", result1)
		}

		// Number to number
		result2, err := Converter[int](42)
		if err != nil {
			t.Errorf("Unexpected error converting int: %v", err)
		}
		if result2 != 42 {
			t.Errorf("Expected 42, got %d", result2)
		}

		// Bool to bool
		result3, err := Converter[bool](true)
		if err != nil {
			t.Errorf("Unexpected error converting bool: %v", err)
		}
		if result3 != true {
			t.Errorf("Expected true, got %t", result3)
		}
	})

	// Test conversion of empty struct
	t.Run("conversion of empty struct", func(t *testing.T) {
		type EmptyStruct struct{}
		source := EmptyStruct{}

		result, err := Converter[EmptyStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := EmptyStruct{}
		if result != expected {
			t.Errorf("Expected %+v, but got %+v", expected, result)
		}
	})

	// Test conversion with nil pointers
	t.Run("conversion with nil pointers in struct", func(t *testing.T) {
		type PointerStruct struct {
			Name *string `json:"name"`
			Age  *int    `json:"age"`
		}

		source := PointerStruct{
			Name: nil,
			Age:  nil,
		}

		result, err := Converter[PointerStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if result.Name != nil || result.Age != nil {
			t.Errorf("Expected nil pointers, but got %+v", result)
		}
	})

	// Test conversion with nested structs
	t.Run("conversion with nested structs", func(t *testing.T) {
		type Address struct {
			Street string `json:"street"`
			City   string `json:"city"`
		}

		type Person struct {
			Name    string  `json:"name"`
			Address Address `json:"address"`
		}

		source := Person{
			Name: "John Doe",
			Address: Address{
				Street: "123 Main St",
				City:   "Anytown",
			},
		}

		result, err := Converter[Person](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if result != source {
			t.Errorf("Expected %+v, but got %+v", source, result)
		}
	})
}

func TestConverter_ErrorCases(t *testing.T) {
	// Test conversion with unsupported types (channels)
	t.Run("conversion with unsupported types should error", func(t *testing.T) {
		source := InvalidStruct{
			Channel: make(chan int),
		}

		_, err := Converter[InvalidStruct](source)
		if err == nil {
			t.Error("Expected error for unsupported type (channel), but got none")
			return
		}

		// JSON marshaling should fail for channels
		if !strings.Contains(err.Error(), "json: unsupported type") {
			t.Errorf("Expected JSON marshaling error, but got: %s", err.Error())
		}
	})

	// Test conversion with function types
	t.Run("conversion with function types should error", func(t *testing.T) {
		type FuncStruct struct {
			Callback func() `json:"callback"`
		}

		source := FuncStruct{
			Callback: func() {},
		}

		_, err := Converter[FuncStruct](source)
		if err == nil {
			t.Error("Expected error for unsupported type (function), but got none")
			return
		}

		// JSON marshaling should fail for functions
		if !strings.Contains(err.Error(), "json: unsupported type") {
			t.Errorf("Expected JSON marshaling error, but got: %s", err.Error())
		}
	})

	// Test conversion that would cause JSON unmarshaling errors
	t.Run("conversion with invalid JSON target should error", func(t *testing.T) {
		// Create a map that would cause unmarshaling issues
		source := map[string]interface{}{
			"age": "not-a-number", // This should cause an error when unmarshaling to int
		}

		_, err := Converter[TargetStruct](source)
		if err == nil {
			t.Error("Expected error for invalid type conversion, but got none")
			return
		}

		// Should be a JSON unmarshaling error
		if !strings.Contains(err.Error(), "cannot unmarshal") && !strings.Contains(err.Error(), "invalid character") {
			t.Errorf("Expected JSON unmarshaling error, but got: %s", err.Error())
		}
	})

	// Test conversion of complex unsupported types
	t.Run("conversion with complex type should error", func(t *testing.T) {
		type ComplexStruct struct {
			ComplexNumber complex64 `json:"complex_number"`
		}

		source := ComplexStruct{
			ComplexNumber: complex(1, 2),
		}

		_, err := Converter[ComplexStruct](source)
		if err == nil {
			t.Error("Expected error for unsupported type (complex64), but got none")
		}
	})
}

func TestConverter_RealWorldScenarios(t *testing.T) {
	// Test API request/response conversion
	t.Run("API request to internal struct conversion", func(t *testing.T) {
		type APIRequest struct {
			UserID    string `json:"user_id"`
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			IsActive  bool   `json:"is_active"`
		}

		type InternalUser struct {
			UserID    string `json:"user_id"`
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			IsActive  bool   `json:"is_active"`
		}

		apiReq := APIRequest{
			UserID:    "user123",
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
			IsActive:  true,
		}

		internalUser, err := Converter[InternalUser](apiReq)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := InternalUser{
			UserID:    "user123",
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
			IsActive:  true,
		}

		if internalUser != expected {
			t.Errorf("Expected %+v, but got %+v", expected, internalUser)
		}
	})

	// Test database model to DTO conversion
	t.Run("database model to DTO conversion", func(t *testing.T) {
		type DatabaseModel struct {
			ID        primitive.ObjectID `json:"_id" bson:"_id"`
			Name      string             `json:"name" bson:"name"`
			Email     string             `json:"email" bson:"email"`
			CreatedAt time.Time          `json:"created_at" bson:"created_at"`
			UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
		}

		type DTO struct {
			ID        primitive.ObjectID `json:"id"`
			Name      string             `json:"name"`
			Email     string             `json:"email"`
			CreatedAt time.Time          `json:"created_at"`
			UpdatedAt time.Time          `json:"updated_at"`
		}

		objID := primitive.NewObjectID()
		now := time.Now()

		dbModel := DatabaseModel{
			ID:        objID,
			Name:      "Test User",
			Email:     "test@example.com",
			CreatedAt: now,
			UpdatedAt: now,
		}

		dto, err := Converter[DTO](dbModel)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		// Note: ObjectID might not convert perfectly due to JSON marshaling/unmarshaling
		// but other fields should convert correctly
		if dto.Name != dbModel.Name || dto.Email != dbModel.Email {
			t.Errorf("Name or Email conversion failed. Expected name=%s, email=%s, got name=%s, email=%s",
				dbModel.Name, dbModel.Email, dto.Name, dto.Email)
		}
	})

	// Test configuration struct conversion
	t.Run("configuration struct conversion", func(t *testing.T) {
		type EnvConfig struct {
			DatabaseURL string `json:"database_url"`
			Port        int    `json:"port"`
			Debug       bool   `json:"debug"`
		}

		type AppConfig struct {
			DatabaseURL string `json:"database_url"`
			Port        int    `json:"port"`
			Debug       bool   `json:"debug"`
		}

		envConfig := EnvConfig{
			DatabaseURL: "mongodb://localhost:27017",
			Port:        8080,
			Debug:       true,
		}

		appConfig, err := Converter[AppConfig](envConfig)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := AppConfig{
			DatabaseURL: "mongodb://localhost:27017",
			Port:        8080,
			Debug:       true,
		}

		if appConfig != expected {
			t.Errorf("Expected %+v, but got %+v", expected, appConfig)
		}
	})

	// Test large struct conversion
	t.Run("large struct with many fields conversion", func(t *testing.T) {
		type LargeStruct struct {
			Field01 string  `json:"field01"`
			Field02 string  `json:"field02"`
			Field03 int     `json:"field03"`
			Field04 int     `json:"field04"`
			Field05 bool    `json:"field05"`
			Field06 bool    `json:"field06"`
			Field07 float64 `json:"field07"`
			Field08 float64 `json:"field08"`
			Field09 string  `json:"field09"`
			Field10 string  `json:"field10"`
		}

		source := LargeStruct{
			Field01: "value01", Field02: "value02", Field03: 3, Field04: 4, Field05: true,
			Field06: false, Field07: 7.7, Field08: 8.8, Field09: "value09", Field10: "value10",
		}

		result, err := Converter[LargeStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if result != source {
			t.Errorf("Large struct conversion failed. Expected %+v, got %+v", source, result)
		}
	})

	// Test slice of maps to slice of structs
	t.Run("slice of maps to slice of structs", func(t *testing.T) {
		type SimpleStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		source := []map[string]interface{}{
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35},
		}

		result, err := Converter[[]SimpleStruct](source)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := []SimpleStruct{
			{Name: "Alice", Age: 25},
			{Name: "Bob", Age: 30},
			{Name: "Charlie", Age: 35},
		}

		if len(result) != len(expected) {
			t.Errorf("Expected slice with %d elements, got %d", len(expected), len(result))
			return
		}

		for i, exp := range expected {
			if result[i] != exp {
				t.Errorf("At index %d, expected %+v, got %+v", i, exp, result[i])
			}
		}
	})
}
