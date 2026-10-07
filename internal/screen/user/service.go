package user

import (
	"context"

	sdk "github.com/friendly-social-ai/golang-sdk"
)

// Service provides logic of other users' profiles.
type Service struct {
	client *sdk.Client
}

// NewService creates new Service from client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) get(user *sdk.Authorization, person sdk.UserDetails) (*sdk.UserProfile, error) {
	return s.client.GetUserDetails(context.Background(), user, person.Id, person.AccessHash)
}

// connect sends a friend request to the person, which accepts theirs when they sent one first.
func (s *Service) connect(user *sdk.Authorization, person sdk.UserDetails) error {
	return s.client.SendFriendRequest(context.Background(), user, person.Id, person.AccessHash)
}

// remove declines the person, which ends the friendship.
func (s *Service) remove(user *sdk.Authorization, person sdk.UserDetails) error {
	return s.client.DeclineFriendRequest(context.Background(), user, person.Id, person.AccessHash)
}
