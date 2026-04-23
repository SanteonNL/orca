package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "should return error code as message",
			code:     "VALIDATION_ERROR",
			expected: "VALIDATION_ERROR",
		},
		{
			name:     "should return empty code",
			code:     "",
			expected: "",
		},
		{
			name:     "should return custom error code",
			code:     "INVALID_RESOURCE_TYPE",
			expected: "INVALID_RESOURCE_TYPE",
		},
		{
			name:     "should return error code with special characters",
			code:     "ERROR_WITH_UNDERSCORE_AND_DASH-123",
			expected: "ERROR_WITH_UNDERSCORE_AND_DASH-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Code: tt.code}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestValidationErrorIsError(t *testing.T) {
	t.Run("should implement error interface", func(t *testing.T) {
		var err error = &Error{Code: "TEST_ERROR"}
		assert.NotNil(t, err)
		assert.Equal(t, "TEST_ERROR", err.Error())
	})
}

func TestValidationErrorZeroValue(t *testing.T) {
	t.Run("should handle zero value", func(t *testing.T) {
		err := Error{}
		assert.Equal(t, "", err.Error())
	})
}

func TestValidatorInterface(t *testing.T) {
	t.Run("should define validator interface correctly", func(t *testing.T) {
		// This test verifies the interface exists and can be implemented
		validator := ConcreteValidator{
			validCodes: map[string]bool{"test": true},
		}
		var v Validator[string] = validator
		assert.NotNil(t, v)
	})
}

func TestValidationErrorMultiple(t *testing.T) {
	t.Run("should handle multiple validation errors", func(t *testing.T) {
		err1 := &Error{Code: "ERROR_1"}
		err2 := &Error{Code: "ERROR_2"}
		err3 := &Error{Code: "ERROR_3"}

		errors := []*Error{err1, err2, err3}
		assert.Len(t, errors, 3)
		assert.Equal(t, "ERROR_1", errors[0].Error())
		assert.Equal(t, "ERROR_2", errors[1].Error())
		assert.Equal(t, "ERROR_3", errors[2].Error())
	})
}

func TestValidationErrorComparison(t *testing.T) {
	t.Run("should compare errors by code", func(t *testing.T) {
		err1 := &Error{Code: "SAME_CODE"}
		err2 := &Error{Code: "SAME_CODE"}
		err3 := &Error{Code: "DIFFERENT_CODE"}

		assert.Equal(t, err1.Error(), err2.Error())
		assert.NotEqual(t, err1.Error(), err3.Error())
	})
}

func TestValidationErrorSlice(t *testing.T) {
	t.Run("should handle slice of validation errors", func(t *testing.T) {
		errors := []*Error{
			{Code: "CODE_1"},
			{Code: "CODE_2"},
			{Code: "CODE_3"},
		}

		// Verify all errors are present
		assert.Len(t, errors, 3)

		// Verify order is preserved
		for i, err := range errors {
			expected := "CODE_" + string(rune(48+i+1)) // "1", "2", "3"
			assert.Equal(t, expected, err.Code)
		}
	})
}

func TestValidationErrorWithNil(t *testing.T) {
	t.Run("should handle nil error slice", func(t *testing.T) {
		var errors []*Error
		assert.Nil(t, errors)
		assert.Len(t, errors, 0)

		errors = []*Error{}
		assert.NotNil(t, errors)
		assert.Len(t, errors, 0)
	})
}

func TestErrorWithVariousCodeFormats(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"uppercase", "ERROR_CODE"},
		{"lowercase", "error_code"},
		{"mixed", "Error_Code"},
		{"with numbers", "ERROR_123"},
		{"with dashes", "ERROR-CODE"},
		{"with dots", "ERROR.CODE"},
		{"empty string", ""},
		{"single character", "E"},
		{"long string", "THIS_IS_A_VERY_LONG_ERROR_CODE_WITH_MANY_PARTS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Code: tt.code}
			assert.Equal(t, tt.code, err.Error())
		})
	}
}

func TestValidationErrorPointer(t *testing.T) {
	t.Run("should handle nil pointer", func(t *testing.T) {
		var err *Error
		assert.Nil(t, err)

		err = &Error{Code: "TEST"}
		assert.NotNil(t, err)
		assert.Equal(t, "TEST", err.Error())
	})
}

func TestValidationErrorMultipleInstances(t *testing.T) {
	t.Run("should create independent error instances", func(t *testing.T) {
		err1 := Error{Code: "ERROR_1"}
		err2 := Error{Code: "ERROR_2"}

		assert.Equal(t, "ERROR_1", err1.Error())
		assert.Equal(t, "ERROR_2", err2.Error())
		assert.NotEqual(t, err1.Error(), err2.Error())
	})
}

type ConcreteValidator struct {
	validCodes map[string]bool
}

func (cv ConcreteValidator) Validate(code string) []*Error {
	errs := []*Error{}
	if code == "" {
		errs = append(errs, &Error{Code: "EMPTY_CODE"})
	}
	if !cv.validCodes[code] && code != "" {
		errs = append(errs, &Error{Code: "INVALID_CODE"})
	}
	return errs
}

func TestConcreteValidatorImplementation(t *testing.T) {
	t.Run("should validate with concrete implementation", func(t *testing.T) {
		var validator Validator[string] = ConcreteValidator{
			validCodes: map[string]bool{
				"valid": true,
				"ok":    true,
			},
		}

		// Test with empty code
		errors := validator.Validate("")
		assert.Len(t, errors, 1)
		assert.Equal(t, "EMPTY_CODE", errors[0].Code)

		// Test with invalid code
		errors = validator.Validate("invalid")
		assert.Len(t, errors, 1)
		assert.Equal(t, "INVALID_CODE", errors[0].Code)

		// Test with valid code
		errors = validator.Validate("valid")
		assert.Len(t, errors, 0)
	})
}

func TestValidatorInterfaceCanBeImplementedByAnyType(t *testing.T) {
	t.Run("should work with different generic types", func(t *testing.T) {
		// Just verify that the interface can be used with different types
		var validator1 Validator[string]
		var validator2 Validator[int]
		var validator3 Validator[float64]

		assert.Nil(t, validator1)
		assert.Nil(t, validator2)
		assert.Nil(t, validator3)
	})
}
