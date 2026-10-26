package utils

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		if name := strings.Split(field.Tag.Get("yaml"), ",")[0]; name != "" {
			return name
		}
		return strings.Split(field.Tag.Get("json"), ",")[0]
	})

	// Compare decimals as floats
	v.RegisterCustomTypeFunc(func(field reflect.Value) any {
		value, _ := field.Interface().(decimal.Decimal).Float64()
		return value
	}, decimal.Decimal{})

	return v
}

func ValidateFields(value any, prefix string) error {
	err := validate.Struct(value)
	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return err
	}

	fieldError := fieldErrors[0]
	name := prefix + fieldError.Field()

	switch fieldError.ActualTag() {
	case "required":
		return fmt.Errorf("missing required %s", name)
	case "gte":
		return fmt.Errorf("%s must be at least %s", name, fieldError.Param())
	case "gt":
		return fmt.Errorf("%s must be greater than %s", name, fieldError.Param())
	case "min":
		return fmt.Errorf("%s must have at least %s characters", name, fieldError.Param())
	case "lte":
		return fmt.Errorf("%s must be at most %s", name, fieldError.Param())
	case "uppercase":
		return fmt.Errorf("%s %q must be uppercase", name, fieldError.Value())
	default:
		return fmt.Errorf("%s fails %s", name, fieldError.ActualTag())
	}
}
