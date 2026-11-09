package profile

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"git.getfriend.ly/ai/cli/internal/screen/auth"
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

// Errors of adding a friend by invite and of confirming an email, worded for the status.
var (
	errInvalidInvite = errors.New("invalid invite link")
	errInviteExpired = errors.New("invite expired")
	errWrongCode     = errors.New("wrong or expired code")
)

// inviteURL prefixes the token in invite links, like createFriendInviteLink in the web.
const inviteURL = "https://invite.getfriend.ly/"

// invite returns the invite link of the user and when it expires. The backend returns the same token until it is
// within an hour of expiring, unless revoke asks for a new one, which stops the old link working.
func (s *Service) invite(user *sdk.Authorization, revoke bool) (string, time.Time, error) {
	generate := s.client.GenerateFriendToken
	if revoke {
		generate = s.client.ForceGenerateFriendToken
	}

	generated, err := generate(context.Background(), user)
	if err != nil {
		return "", time.Time{}, err
	}

	return inviteURL + generated.Token.Value(), generated.Expiration, nil
}

// lookup returns the token of invite, a link or a bare token, and the profile of its owner.
func (s *Service) lookup(invite string) (sdk.FriendToken, *sdk.FriendTokenOwner, error) {
	token, err := parseInvite(invite)
	if err != nil {
		return sdk.FriendToken{}, nil, err
	}

	owner, err := s.client.GetFriendTokenOwner(context.Background(), token)
	var apiErr sdk.APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
		return sdk.FriendToken{}, nil, errInvalidInvite
	}

	return token, owner, err
}

// add adds the owner of token to the friends of the user.
func (s *Service) add(user *sdk.Authorization, token sdk.FriendToken) error {
	err := s.client.AddFriend(context.Background(), user, token)
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

// link emails a code that confirm takes to bind email to the user. The backend sends no new code while an earlier
// one to email is still active.
func (s *Service) link(user *sdk.Authorization, email string) error {
	address, err := sdk.NewEmail(strings.TrimSpace(email))
	if err != nil {
		return fmt.Errorf("profile: failed to create email: %w", err)
	}

	return s.client.LinkEmail(context.Background(), user, address, auth.Locale)
}

// confirm checks code and binds the email that link sent it to, replacing any bound email.
func (s *Service) confirm(user *sdk.Authorization, code string) error {
	emailCode, err := sdk.NewEmailCode(strings.TrimSpace(code))
	if err != nil {
		return fmt.Errorf("profile: failed to create code: %w", err)
	}

	// the backend answers 403 to a wrong code, an expired code and a code out of attempts alike
	err = s.client.ConfirmEmail(context.Background(), user, emailCode)
	var apiErr sdk.APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusForbidden {
		return errWrongCode
	}

	return err
}

// unlink removes the email bound to the user.
func (s *Service) unlink(user *sdk.Authorization) error {
	return s.client.UnlinkEmail(context.Background(), user)
}
