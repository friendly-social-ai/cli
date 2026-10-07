package register

import (
	"context"
	"fmt"
	"strings"

	"github.com/friendly-social-ai/cli/internal/screen/auth"
	sdk "github.com/friendly-social-ai/golang-sdk"
)

// Service provides registration logic.
type Service struct {
	client *sdk.Client
}

// NewService creates Service from sdk.Client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) register(nicknameString, descriptionString, interestsString, socialString string) (*sdk.Authorization, error) {
	nickname, err := sdk.NewNickname(nicknameString)
	if err != nil {
		return nil, fmt.Errorf("register: failed to create nickname: %w", err)
	}

	description, err := sdk.NewUserDescription(descriptionString)
	if err != nil {
		return nil, fmt.Errorf("register: failed to create description: %w", err)
	}

	// interests are optional, as on the web
	var interestsSlice []sdk.Interest
	if strings.TrimSpace(interestsString) != "" {
		for interestStr := range strings.SplitSeq(interestsString, ",") {
			interest, err := sdk.NewInterest(strings.TrimSpace(interestStr))
			if err != nil {
				return nil, fmt.Errorf("register: failed to create interest: %w", err)
			}

			interestsSlice = append(interestsSlice, interest)
		}
	}

	interests, err := sdk.NewInterests(interestsSlice...)
	if err != nil {
		return nil, fmt.Errorf("register: failed to create interests: %w", err)
	}

	// social link is optional, zero value is sent as null
	var socialLink sdk.SocialLink
	if strings.TrimSpace(socialString) != "" {
		socialLink, err = sdk.NewSocialLink(socialString)
		if err != nil {
			return nil, fmt.Errorf("register: failed to create social link: %w", err)
		}
	}

	user, err := s.client.Register(context.Background(), nickname, description, interests, nil, socialLink)
	if err != nil {
		return nil, fmt.Errorf("register: failed to register: %w", err)
	}

	err = auth.Save(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}
