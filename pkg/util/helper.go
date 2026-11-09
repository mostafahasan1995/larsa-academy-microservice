package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"math/rand"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Identifiable interface {
	GetId() string
}

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func Paginate(r *http.Request) (sk int64, lim int64, er error) {
	skipStr := r.URL.Query().Get("skip")
	limitStr := r.URL.Query().Get("limit")

	if skipStr != "" || limitStr != "" {
		var skip, limit int64
		if skipStr != "" {
			skip, _ = strconv.ParseInt(skipStr, 10, 64)
		}
		if limitStr != "" {
			limit, _ = strconv.ParseInt(limitStr, 10, 64)
		} else {
			limit = 10
		}
		return skip, limit, nil
	}

	page := r.URL.Query().Get("page")
	perPage := r.URL.Query().Get("perPage")

	var skip, limit int64
	var convErr error
	if perPage == "" {
		limit = 40
	} else {
		limit, convErr = strconv.ParseInt(perPage, 10, 64)
		if convErr != nil {
			return 0, 0, convErr
		}
	}
	if page == "" {
		skip = 0
	} else {
		skip, convErr = strconv.ParseInt(page, 10, 64)
		if convErr != nil {
			return 0, 0, convErr
		}
		skip = (skip - 1) * limit
	}

	return skip, limit, nil
}

func InSlice[T comparable](ele T, s []T) bool {
	for _, e := range s {
		if e == ele {
			return true
		}
	}

	return false

}

func GetBsonFromString(query string) (bson.M, error) {
	var bs bson.M = bson.M{}
	if query != "" {
		err := bson.UnmarshalExtJSON([]byte(query), true, &bs)
		if err != nil {
			return nil, err
		}
	}

	return bs, nil
}

func GeneratePassword(passWordLength, minSpecialChar, minUpper, minNum int) string {
	var (
		lowerCharSet   = "abcdedfghijklmnopqrst"
		upperCharSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		specialCharSet = "!@#$%&*"
		numberSet      = "0123456789"
		allCharSet     = lowerCharSet + upperCharSet + specialCharSet + numberSet
	)
	var password strings.Builder
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	for i := 0; i < minSpecialChar; i++ {
		index := r.Intn(len(specialCharSet))
		password.WriteString(string(specialCharSet[index]))
	}

	for i := 0; i < minUpper; i++ {
		index := r.Intn(len(upperCharSet))
		password.WriteString(string(upperCharSet[index]))
	}

	for i := 0; i < minNum; i++ {
		index := r.Intn(len(numberSet))
		password.WriteString(string(numberSet[index]))
	}

	remainLength := passWordLength - minSpecialChar - minUpper - minNum

	for i := 0; i < remainLength; i++ {
		index := r.Intn(len(allCharSet))
		password.WriteString(string(allCharSet[index]))
	}

	return password.String()

}

type ReqArgs struct {
	Sort    bson.M
	GroupBy bson.M
	//any other args needed
}

func (a *ReqArgs) SetSort(r *http.Request) error {
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	if sortBy != "" && sortOrder != "" {
		sort := bson.M{}
		order, convErr := strconv.ParseInt(sortOrder, 10, 64)
		if convErr != nil {
			return convErr
		}
		sort[sortBy] = order
		a.Sort = sort
	}

	return nil
}

func (a *ReqArgs) SetGroupBy(r *http.Request) error {
	groupBy := r.URL.Query().Get("groupBy")

	if groupBy != "" {
		group := bson.M{
			"_id":     "$" + groupBy,
			"records": bson.M{"$push": "$$ROOT"},
		}

		a.GroupBy = group
	}

	return nil

}

func Contains[T comparable](arr []T, key T) bool {
	for _, value := range arr {
		if value == key {
			return true
		}
	}
	return false
}

func SliceFilter[T any](arr []T, callBack func(el T) bool) []T {
	var result []T
	for _, e := range arr {
		ok := callBack(e)
		if ok {
			result = append(result, e)
		}
	}

	return result
}

func SliceFind[T any](arr []T, callback func(el T) bool) (T, bool) {
	var result T
	var ok bool
	for _, e := range arr {
		ok = callback(e)
		if ok {
			result = e
			break
		}
	}

	return result, ok
}

func RoundToDecimalPlaces(value float64, decimalPlaces int) float64 {
	factor := math.Pow(10, float64(decimalPlaces))
	return math.Round(value*factor) / factor
}

func IsValidEmail(email string) bool {
	// Define the regex pattern for a valid email address
	const emailPattern = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`

	// Compile the regex pattern
	re := regexp.MustCompile(emailPattern)

	// Match the email address against the pattern
	return re.MatchString(email)
}

func GenerateUniqueString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func HasDuplicateIDs[T Identifiable](slice []T) bool {
	seen := make(map[string]bool)
	for _, item := range slice {
		if seen[item.GetId()] {
			return true
		}
		seen[item.GetId()] = true
	}
	return false
}

func SliceReorder[T Identifiable](s []T, oldIndex, newIndex int) ([]T, error) {
	if oldIndex >= 0 && oldIndex < len(s) && newIndex >= 0 && newIndex < len(s) {
		// Remove the element from the old position
		ele := s[oldIndex]
		s = append(s[:oldIndex], s[oldIndex+1:]...)
		// Insert the element at the new position
		s = append(s[:newIndex], append([]T{ele}, s[newIndex:]...)...)
		return s, nil
	} else {
		return s, errors.New("invalid indices")
	}
}

func InsertAt[T any](slice []T, index int, element T) ([]T, error) {
	// Validate the index
	if index < 0 || index > len(slice) {
		return nil, fmt.Errorf("index out of range: %d", index)
	}

	// Insert the element at the specified index
	newSlice := append(slice[:index], append([]T{element}, slice[index:]...)...)

	return newSlice, nil
}

func ReplacePlaceholders(text string, data map[string]any) (string, error) {

	temp := new(bytes.Buffer)

	tmpl, err := template.New("t").Parse(text)
	if err != nil {
		return "", err
	}

	if err := tmpl.Execute(temp, data); err != nil {
		return "", err
	}

	return temp.String(), nil
}

func Converter[To any, From any](from From) (To, error) {

	var to To

	b, err := json.Marshal(from)
	if err != nil {
		return to, err
	}

	if err := json.Unmarshal(b, &to); err != nil {
		return to, err
	}

	return to, nil
}

// ReplacePlaceholdersInStruct applies placeholder replacement to all string fields in a struct
func ReplacePlaceholdersInStruct[T any](input *T, data map[string]any) error {
	val := reflect.ValueOf(input)

	// Ensure we have a pointer to a struct
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return fmt.Errorf("input must be a pointer to a struct")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("input must be a pointer to a struct")
	}

	typ := val.Type()

	// Iterate through all fields
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if fieldType.PkgPath != "" {
			continue
		}

		// Only process string fields that are settable
		if field.Kind() == reflect.String && field.CanSet() {
			originalValue := field.String()

			// Apply placeholder replacement
			newValue, err := ReplacePlaceholders(originalValue, data)
			if err != nil {
				return fmt.Errorf("error replacing placeholders in field %s: %w", fieldType.Name, err)
			}

			field.SetString(newValue)
		}
	}

	return nil
}

// for test purpose only
func ConvertStringsToObjectIDs(data map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range data {
		if str, ok := v.(string); ok {
			// Try to convert to ObjectID first
			if id, err := primitive.ObjectIDFromHex(str); err == nil {
				result[k] = id
				continue
			}

			// Try to convert to time.Time for various date formats
			if dateTime, err := parseDateTime(str); err == nil {
				result[k] = dateTime
				continue
			}
		}

		result[k] = v
	}

	return result
}

// for test purpose only
// parseDateTime attempts to parse a string as a date/time in various common formats
func parseDateTime(str string) (time.Time, error) {
	// Common date/time formats to try
	formats := []string{
		time.RFC3339,              // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,          // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05Z",    // ISO 8601 with Z
		"2006-01-02T15:04:05",     // ISO 8601 without timezone
		"2006-01-02 15:04:05",     // Common MySQL datetime format
		"2006-01-02",              // Date only
		"2006/01/02",              // Alternative date format
		"01/02/2006",              // US date format
		"02/01/2006",              // EU date format
		"2006-01-02 15:04:05.000", // With milliseconds
	}

	for _, format := range formats {
		if t, err := time.Parse(format, str); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", str)
}
func GenerateInvoiceNumber() string {
	return fmt.Sprintf("INV-%s", time.Now().Format("20060102150405"))
}

func SliceChunk[T any](arr []T, size int) [][]T {
	if size == 0 {
		return [][]T{arr}
	}
	c := 0
	result := [][]T{}
	for {
		var a []T
		if c+size > len(arr) {
			a = arr[c:]
		} else {
			a = arr[c : c+size]
		}
		c += len(a)
		result = append(result, a)
		if c == len(arr) {
			break
		}
	}

	return result

}

func Parse24h(hour string) (time.Time, error) {
	layout := "15:04"
	parsedTime, err := time.Parse(layout, hour)
	if err != nil {
		return time.Time{}, err
	}

	return parsedTime, nil
}
