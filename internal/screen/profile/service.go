package profile

import (
	"context"
	"fmt"
	"slices"
	"strings"

	sdk "github.com/friendly-social/golang-sdk"
)

// Service provides logic of retrieving profile data.
type Service struct {
	client *sdk.Client
}

// NewService creates new Service from client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) get(user *sdk.Authorization) (*sdk.UserDetails, error) {
	details, err := s.client.GetSelfDetails(context.Background(), user)
	if err != nil {
		return nil, err
	}

	return details, nil
}

// edit saves the fields that differ from self. Interests are separated by commas. Empty interests and social link stay
// as they are, since the SDK has no way to clear them.
func (s *Service) edit(user *sdk.Authorization, self *sdk.UserDetails, nickname, description, interests, social string) error {
	newNickname, err := sdk.NewNickname(nickname)
	if err != nil {
		return fmt.Errorf("profile: failed to create nickname: %w", err)
	}

	newDescription, err := sdk.NewUserDescription(description)
	if err != nil {
		return fmt.Errorf("profile: failed to create description: %w", err)
	}

	opts := appendIf(nil, newNickname.Value() != self.Nickname.Value(), sdk.EditNicknameOption(newNickname))
	opts = appendIf(opts, newDescription.Value() != self.Description.Value(), sdk.EditDescriptionOption(newDescription))

	if strings.TrimSpace(interests) != "" {
		var interestsSlice []sdk.Interest
		for interestStr := range strings.SplitSeq(interests, ",") {
			interest, err := sdk.NewInterest(strings.TrimSpace(interestStr))
			if err != nil {
				return fmt.Errorf("profile: failed to create interest: %w", err)
			}

			interestsSlice = append(interestsSlice, interest)
		}

		newInterests, err := sdk.NewInterests(interestsSlice...)
		if err != nil {
			return fmt.Errorf("profile: failed to create interests: %w", err)
		}

		same := slices.EqualFunc(interestsSlice, self.Interests.Value(), func(a, b sdk.Interest) bool {
			return a.Value() == b.Value()
		})
		opts = appendIf(opts, !same, sdk.EditInterestsOption(newInterests))
	}

	if strings.TrimSpace(social) != "" {
		link, err := sdk.NewSocialLink(social)
		if err != nil {
			return fmt.Errorf("profile: failed to create social link: %w", err)
		}

		opts = appendIf(opts, link.Value() != self.SocialLink.Value(), sdk.EditSocialLinkOption(link))
	}

	return s.client.EditAccount(context.Background(), user, opts...)
}

// appendIf appends opt to opts when changed. It is generic, so it can hold the option type that the SDK doesn't export.
func appendIf[T any](opts []T, changed bool, opt T) []T {
	if changed {
		return append(opts, opt)
	}

	return opts
}
