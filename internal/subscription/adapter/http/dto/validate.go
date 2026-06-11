package dto

import (
	"errors"
	"fmt"

	"github-release-notifier/internal/shared/github"

	"github.com/go-playground/validator/v10"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.RegisterValidation("reponame", validateRepoName); err != nil {
		panic("register reponame validation: " + err.Error())
	}

	return v
}

func validateRepoName(fl validator.FieldLevel) bool {
	_, _, err := github.ParseRepo(fl.Field().String())
	return err == nil
}

func Validate(s any) error {
	if err := validate.Struct(s); err != nil {
		return fmt.Errorf("validate request: %w", err)
	}

	return nil
}

func ValidateEmail(email string) error {
	if err := validate.Var(email, "required,email"); err != nil {
		return fmt.Errorf("validate email: %w", err)
	}

	return nil
}

func ValidationMessage(err error) string {
	if ve, ok := errors.AsType[validator.ValidationErrors](err); ok && len(ve) > 0 {
		switch ve[0].Field() {
		case "Email":
			return "invalid email format"
		case "Repo":
			return "invalid repo format, expected owner/repo"
		}
	}

	return "invalid request"
}
