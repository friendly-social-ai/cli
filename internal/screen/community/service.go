package community

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"

	sdk "github.com/friendly-social/golang-sdk"
)

// maxImageBytes limits size of downloaded post images.
const maxImageBytes = 20 << 20

// Service provides logic of reading and writing community posts.
type Service struct {
	client *sdk.Client
	http   *http.Client
}

// NewService creates new Service from client.
func NewService(client *sdk.Client) *Service {
	return &Service{
		client: client,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) list(user *sdk.Authorization, cursor *sdk.CursorId) (*sdk.Cursor[sdk.CommunityPost], error) {
	return s.client.ListCommunity(context.Background(), user, cursor)
}

func (s *Service) details(user *sdk.Authorization, post sdk.CommunityPostDescriptor) (*sdk.CommunityPostDetails, error) {
	return s.client.GetCommunityPost(context.Background(), user, post)
}

func (s *Service) replies(user *sdk.Authorization, post sdk.CommunityPostDescriptor, cursor *sdk.CursorId) (*sdk.Cursor[sdk.CommunityPostReply], error) {
	return s.client.GetCommunityReplies(context.Background(), user, post, cursor)
}

func (s *Service) post(user *sdk.Authorization, text string, replyTo *sdk.CommunityPostDescriptor) error {
	postText, err := sdk.NewCommunityPostText(text)
	if err != nil {
		return err
	}

	_, err = s.client.PostCommunity(context.Background(), user, postText, replyTo)
	return err
}

func (s *Service) edit(user *sdk.Authorization, id sdk.CommunityPostId, text string) error {
	postText, err := sdk.NewCommunityPostText(text)
	if err != nil {
		return err
	}

	return s.client.EditCommunityPost(context.Background(), user, id, postText)
}

func (s *Service) delete(user *sdk.Authorization, id sdk.CommunityPostId) error {
	return s.client.DeleteCommunityPost(context.Background(), user, id)
}

func (s *Service) image(url string) (image.Image, error) {
	resp, err := s.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}
