package profile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	sdk "git.getfriend.ly/ai/golang-sdk"
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

// edit saves the fields that differ from self. Interests are separated by commas. An empty interests or social link
// field clears that field.
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

	var interestsSlice []sdk.Interest
	if strings.TrimSpace(interests) != "" {
		for interestStr := range strings.SplitSeq(interests, ",") {
			interest, err := sdk.NewInterest(strings.TrimSpace(interestStr))
			if err != nil {
				return fmt.Errorf("profile: failed to create interest: %w", err)
			}

			interestsSlice = append(interestsSlice, interest)
		}
	}

	newInterests, err := sdk.NewInterests(interestsSlice...)
	if err != nil {
		return fmt.Errorf("profile: failed to create interests: %w", err)
	}

	sameInterests := slices.EqualFunc(interestsSlice, self.Interests.Value(), func(a, b sdk.Interest) bool {
		return a.Value() == b.Value()
	})
	opts = appendIf(opts, !sameInterests, sdk.EditInterestsOption(newInterests))

	// the SDK sends a zero social link as null, which removes it
	var link sdk.SocialLink
	if strings.TrimSpace(social) != "" {
		link, err = sdk.NewSocialLink(social)
		if err != nil {
			return fmt.Errorf("profile: failed to create social link: %w", err)
		}
	}

	opts = appendIf(opts, link.Value() != self.SocialLink.Value(), sdk.EditSocialLinkOption(link))

	return s.client.EditAccount(context.Background(), user, opts...)
}

// appendIf appends opt to opts when changed. It is generic, so it can hold the option type that the SDK doesn't export.
func appendIf[T any](opts []T, changed bool, opt T) []T {
	if changed {
		return append(opts, opt)
	}

	return opts
}

// friends returns friends of the user.
func (s *Service) friends(user *sdk.Authorization) ([]sdk.UserDetails, error) {
	network, err := s.client.GetNetworkDetails(context.Background(), user)
	if err != nil {
		return nil, err
	}

	return network.Friends, nil
}

// Errors of adding a friend by invite, worded for the status.
var (
	errInvalidInvite = errors.New("invalid invite link")
	errInviteExpired = errors.New("invite expired")
)

// inviteURL prefixes the token in invite links, like createFriendInviteLink in the web.
const inviteURL = "https://invite.getfriend.ly/"

// invite returns the invite link of the user and when it expires. The backend returns the same token until it is
// within an hour of expiring.
func (s *Service) invite(user *sdk.Authorization) (string, time.Time, error) {
	generated, err := s.client.GenerateFriendToken(context.Background(), user)
	if err != nil {
		return "", time.Time{}, err
	}

	return inviteURL + generated.Token.Value(), generated.Expiration, nil
}

// add adds the owner of invite, a link or a bare token, to the friends of the user.
func (s *Service) add(user *sdk.Authorization, invite string) error {
	token, err := parseInvite(invite)
	if err != nil {
		return err
	}

	err = s.client.AddFriend(context.Background(), user, token)
	if errors.Is(err, sdk.ErrFriendTokenExpired) {
		return errInviteExpired
	}

	return err
}

// parseInvite returns the token that ends invite. That covers a bare token, an invite link and the older
// getfriend.ly/#?reference=add/<token> link.
func parseInvite(invite string) (sdk.FriendToken, error) {
	invite = strings.TrimSuffix(strings.TrimSpace(invite), "/")
	token, err := sdk.NewFriendToken(invite[strings.LastIndex(invite, "/")+1:])
	if err != nil {
		return sdk.FriendToken{}, fmt.Errorf("profile: failed to read invite: %w", errInvalidInvite)
	}

	return token, nil
}
