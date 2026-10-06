package people

import (
	"context"

	sdk "github.com/friendly-social/golang-sdk"
)

// Service provides logic of retrieving people data.
type Service struct {
	client *sdk.Client
}

// NewService creates new Service from client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) get(user *sdk.Authorization) ([]sdk.FeedEntry, error) {
	queue, err := s.client.GetFeedQueue(context.Background(), user)
	if err != nil {
		return nil, err
	}

	return queue.Entries, nil
}

// connect sends a friend request to the person, which accepts theirs when they sent one first.
func (s *Service) connect(user *sdk.Authorization, person sdk.UserDetails) error {
	return s.client.SendFriendRequest(context.Background(), user, person.Id, person.AccessHash)
}

// skip declines the person, so they leave the queue.
func (s *Service) skip(user *sdk.Authorization, person sdk.UserDetails) error {
	return s.client.DeclineFriendRequest(context.Background(), user, person.Id, person.AccessHash)
}
