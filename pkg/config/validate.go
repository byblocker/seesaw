package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

var settingsValidator = newSettingsValidator()

func newSettingsValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		return strings.Split(field.Tag.Get("yaml"), ",")[0]
	})

	// Compare decimals as floats
	v.RegisterCustomTypeFunc(func(field reflect.Value) any {
		value, _ := field.Interface().(decimal.Decimal).Float64()
		return value
	}, decimal.Decimal{})

	return v
}

func validateSettings(settings any, section string) error {
	return describeFieldError(settingsValidator.Struct(settings), func(fieldError validator.FieldError) string {
		return section + "." + fieldError.Field()
	})
}

func validateFields(value any) error {
	return describeFieldError(settingsValidator.Struct(value), validator.FieldError.Field)
}

func validateAssets(assets []AssetPlan) error {
	return describeFieldError(settingsValidator.Var(assets, "dive"), func(fieldError validator.FieldError) string {
		return "assets" + fieldError.Namespace()
	})
}

func describeFieldError(err error, nameOf func(validator.FieldError) string) error {
	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return err
	}

	fieldError := fieldErrors[0]
	name := nameOf(fieldError)

	switch fieldError.ActualTag() {
	case "required":
		return fmt.Errorf("missing required %s", name)
	case "gte":
		return fmt.Errorf("%s must be at least %s", name, fieldError.Param())
	case "gt":
		return fmt.Errorf("%s must be greater than %s", name, fieldError.Param())
	case "lte":
		return fmt.Errorf("%s must be at most %s", name, fieldError.Param())
	case "uppercase":
		return fmt.Errorf("%s %q must be uppercase", name, fieldError.Value())
	default:
		return fmt.Errorf("%s fails %s", name, fieldError.ActualTag())
	}
}
