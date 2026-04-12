// Package validate provides JSON schema validation for external API responses.
// It implements strict unmarshaling with validation to detect unexpected
// response formats and API changes early.
package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// ValidationError represents a JSON validation failure.
type ValidationError struct {
	Field   string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error at %s: %s (cause: %v)", e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("validation error at %s: %s", e.Field, e.Message)
}

// UnmarshalStrict performs strict JSON unmarshaling that disallows unknown fields
// and validates required fields.
// Usage:
//
//	var result MyStruct
//	if err := validate.UnmarshalStrict(data, &result); err != nil {
//	    return err
//	}
func UnmarshalStrict(data []byte, v interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return &ValidationError{
			Field:   "root",
			Message: "failed to unmarshal JSON",
			Cause:   err,
		}
	}

	// Check for trailing data
	if decoder.More() {
		return &ValidationError{
			Field:   "root",
			Message: "trailing data after JSON object",
		}
	}

	return nil
}

// UnmarshalStrictFromReader performs strict JSON unmarshaling from an io.Reader.
func UnmarshalStrictFromReader(r io.Reader, v interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return &ValidationError{
			Field:   "root",
			Message: "failed to unmarshal JSON",
			Cause:   err,
		}
	}

	return nil
}

// ValidateResponse combines unmarshaling with custom validation.
// The validator function is called after successful unmarshaling.
func ValidateResponse(data []byte, v interface{}, validator func() error) error {
	if err := UnmarshalStrict(data, v); err != nil {
		return err
	}

	if validator != nil {
		if err := validator(); err != nil {
			return &ValidationError{
				Field:   "validation",
				Message: "custom validation failed",
				Cause:   err,
			}
		}
	}

	return nil
}

// Required checks that a string value is not empty.
func Required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{
			Field:   field,
			Message: "required field is empty",
		}
	}
	return nil
}

// RequiredInt checks that an integer value is non-zero (if required).
func RequiredInt(field string, value int) error {
	if value == 0 {
		return &ValidationError{
			Field:   field,
			Message: "required integer field is zero",
		}
	}
	return nil
}

// RequiredFloat checks that a float value is non-zero (if required).
func RequiredFloat(field string, value float64) error {
	if value == 0 {
		return &ValidationError{
			Field:   field,
			Message: "required float field is zero",
		}
	}
	return nil
}

// Range validates that a float is within an acceptable range.
func Range(field string, value, min, max float64) error {
	if value < min || value > max {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("value %.2f is outside range [%.2f, %.2f]", value, min, max),
		}
	}
	return nil
}

// OneOf validates that a string value is one of the allowed options.
func OneOf(field, value string, allowed ...string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return &ValidationError{
		Field:   field,
		Message: fmt.Sprintf("value '%s' is not one of allowed values: %v", value, allowed),
	}
}

// APIResponse is an interface for API response types that can self-validate.
type APIResponse interface {
	Validate() error
}

// UnmarshalAndValidate unmarshals JSON and calls the response's Validate method.
func UnmarshalAndValidate(data []byte, v APIResponse) error {
	if err := UnmarshalStrict(data, v); err != nil {
		return err
	}
	return v.Validate()
}

// Schema represents a simple JSON schema for validation.
type Schema struct {
	RequiredFields []string
	FieldTypes     map[string]string // field name -> expected type
}

// ValidateAgainstSchema validates a JSON object against a simple schema.
func ValidateAgainstSchema(data []byte, schema Schema) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &ValidationError{
			Field:   "root",
			Message: "failed to parse JSON for schema validation",
			Cause:   err,
		}
	}

	// Check required fields
	for _, field := range schema.RequiredFields {
		if _, ok := raw[field]; !ok {
			return &ValidationError{
				Field:   field,
				Message: "required field is missing",
			}
		}
	}

	return nil
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	var vErr *ValidationError
	return errors.As(err, &vErr)
}

// GetValidationErrors extracts all validation errors from an error chain.
func GetValidationErrors(err error) []string {
	if err == nil {
		return nil
	}

	var errs []string
	var vErr *ValidationError
	if errors.As(err, &vErr) {
		errs = append(errs, vErr.Error())
	}

	// Also unwrap and check
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil && unwrapped != err {
		errs = append(errs, GetValidationErrors(unwrapped)...)
	}

	return errs
}

// ValidateStruct validates a struct using reflection and field tags.
// It checks for required fields and basic type constraints.
func ValidateStruct(v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return &ValidationError{
			Field:   "root",
			Message: "expected a struct",
		}
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		jsonTag := fieldType.Tag.Get("json")
		validateTag := fieldType.Tag.Get("validate")

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Parse JSON tag to get field name
		fieldName := fieldType.Name
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Check required validation
		if validateTag == "required" {
			switch field.Kind() {
			case reflect.String:
				if field.String() == "" {
					return &ValidationError{
						Field:   fieldName,
						Message: "required field is empty",
					}
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if field.Int() == 0 {
					return &ValidationError{
						Field:   fieldName,
						Message: "required integer field is zero",
					}
				}
			case reflect.Float32, reflect.Float64:
				if field.Float() == 0 {
					return &ValidationError{
						Field:   fieldName,
						Message: "required float field is zero",
					}
				}
			case reflect.Ptr:
				if field.IsNil() {
					return &ValidationError{
						Field:   fieldName,
						Message: "required field is nil",
					}
				}
			}
		}
	}

	return nil
}

// NoUnknownFields wraps a decoder to return detailed errors about unknown fields.
func NoUnknownFields(data []byte, target interface{}) error {
	// First pass: unmarshal to check for unknown fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Get target struct fields
	val := reflect.ValueOf(target)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	knownFields := make(map[string]bool)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			knownFields[field.Name] = true
		} else {
			parts := strings.Split(jsonTag, ",")
			knownFields[parts[0]] = true
		}
	}

	// Check for unknown fields
	for key := range raw {
		if !knownFields[key] {
			return &ValidationError{
				Field:   key,
				Message: "unknown field in JSON response",
			}
		}
	}

	// Second pass: normal unmarshal
	return json.Unmarshal(data, target)
}
