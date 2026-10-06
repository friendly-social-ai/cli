package activity

import (
	"context"

	sdk "github.com/friendly-social/golang-sdk"
)

// Service provides logic of reading activity.
type Service struct {
	client *sdk.Client
}

// NewService creates new Service from client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) list(user *sdk.Authorization, cursor *sdk.CursorId) (*sdk.Cursor[sdk.Activity], error) {
	return s.client.ListActivity(context.Background(), user, cursor)
}

func (s *Service) read(user *sdk.Authorization, id sdk.ActivityId) error {
	return s.client.ReadActivity(context.Background(), user, id)
}
