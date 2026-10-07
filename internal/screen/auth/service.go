package auth

import (
	"context"
	"fmt"

	sdk "github.com/friendly-social-ai/golang-sdk"
)

// locale is the language of login e-mail. Backend accepts either "en" or "ru".
var locale = sdk.NewEmailLocale("en")

// Service provides e-mail login logic.
type Service struct {
	client *sdk.Client
}

// NewService creates Service from sdk.Client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) send(emailString string) error {
	email, err := sdk.NewEmail(emailString)
	if err != nil {
		return fmt.Errorf("auth: failed to create email: %w", err)
	}

	err = s.client.SendLoginRequest(context.Background(), email, locale)
	if err != nil {
		return fmt.Errorf("auth: failed to send login code: %w", err)
	}

	return nil
}

func (s *Service) confirm(emailString, codeString string) (*sdk.Authorization, error) {
	email, err := sdk.NewEmail(emailString)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to create email: %w", err)
	}

	code, err := sdk.NewEmailCode(codeString)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to create code: %w", err)
	}

	user, err := s.client.ConfirmLogin(context.Background(), email, code)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to login: %w", err)
	}

	err = Save(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}
