package types

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// Nullable represents a value that can be explicitly set to null.
// It tracks whether the value was explicitly null vs. not provided.
type Nullable[T any] struct {
	Value          *T
	IsExplicitNull bool // true when JSON value was explicitly null
}

// UnmarshalJSON implements json.Unmarshaler
// It distinguishes between:
// - Field not provided (Value = nil, IsExplicitNull = false)
// - Field explicitly null (Value = nil, IsExplicitNull = true)
func (n *Nullable[T]) UnmarshalJSON(data []byte) error {
	// Check if the JSON value is explicitly null
	if string(data) == "null" {
		n.IsExplicitNull = true
		n.Value = nil
		return nil
	}

	// Otherwise, unmarshal the value normally
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = &value
	n.IsExplicitNull = false
	return nil
}

// MarshalJSON implements json.Marshaler (value receiver to ensure it is invoked for value fields)
func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if n.IsExplicitNull || n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

// MarshalBSONValue implements bson.ValueMarshaler so the stored value is either null or the underlying value.
func (n Nullable[T]) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if n.Value == nil {
		return bsontype.Null, nil, nil
	}
	return bson.MarshalValue(*n.Value)
}

// UnmarshalBSONValue implements bson.ValueUnmarshaler to load into Nullable.
func (n *Nullable[T]) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	if t == bsontype.Null {
		n.Value = nil
		n.IsExplicitNull = true
		return nil
	}
	var v T
	if err := bson.UnmarshalValue(t, data, &v); err != nil {
		return err
	}
	n.Value = &v
	n.IsExplicitNull = false
	return nil
}

// GetValue returns the underlying value or nil
func (n *Nullable[T]) GetValue() *T {
	return n.Value
}

// IsNull returns true if the value is null (either explicitly or not provided)
func (n *Nullable[T]) IsNull() bool {
	return n.Value == nil
}

// ShouldSetToNull returns true if the field should be set to null in MongoDB (explicitly null)
func (n *Nullable[T]) ShouldSetToNull() bool {
	return n.IsExplicitNull
}

// IsProvided returns true if the field was provided in the JSON (either with a value or explicitly null)
func (n *Nullable[T]) IsProvided() bool {
	return n.IsExplicitNull || n.Value != nil
}

// IsExplicitlyNull returns true if the field was explicitly set to null in JSON
func (n *Nullable[T]) IsExplicitlyNull() bool {
	return n.IsExplicitNull
}
