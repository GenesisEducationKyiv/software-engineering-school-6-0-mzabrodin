package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

var eventValidator = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			return f.Name
		}

		return name
	})

	if err := v.RegisterValidation("reponame", func(fl validator.FieldLevel) bool {
		return repoNamePattern.MatchString(fl.Field().String())
	}); err != nil {
		panic(fmt.Sprintf("events: register reponame validator: %v", err))
	}

	return v
}

func Marshal[T any](e T) ([]byte, error) {
	if err := eventValidator.Struct(e); err != nil {
		return nil, fmt.Errorf("validate event %T: %w", e, err)
	}

	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal event %T: %w", e, err)
	}

	return data, nil
}

func Unmarshal[T any](data []byte) (T, error) {
	var e T

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&e); err != nil {
		return e, fmt.Errorf("unmarshal event %T: %w", e, err)
	}

	if err := eventValidator.Struct(e); err != nil {
		return e, fmt.Errorf("validate event %T: %w", e, err)
	}

	return e, nil
}
