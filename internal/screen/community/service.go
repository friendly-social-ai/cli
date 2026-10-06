package community

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/friendly-social/golang-sdk"
)

const (
	// maxImageBytes limits size of downloaded post images.
	maxImageBytes = 20 << 20

	// maxUploadBytes limits size of attached images, same as web.
	maxUploadBytes = 5_000_000
)

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

func (s *Service) post(user *sdk.Authorization, text string, replyTo *sdk.CommunityPostDescriptor) (*sdk.CommunityPostDescriptor, error) {
	postText, err := sdk.NewCommunityPostText(text)
	if err != nil {
		return nil, err
	}

	return s.client.PostCommunity(context.Background(), user, postText, replyTo)
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

// download fetches image at url, up to maxImageBytes.
func (s *Service) download(url string) ([]byte, error) {
	resp, err := s.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	return data, nil
}

func (s *Service) image(url string) (image.Image, error) {
	data, err := s.download(url)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}

// upload sends image at path typed or dropped onto the terminal and returns its URL for embedding into a post.
func (s *Service) upload(user *sdk.Authorization, path string) (string, error) {
	path, err := cleanPath(path)
	if err != nil {
		return "", err
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	if info.Size() > maxUploadBytes {
		return "", fmt.Errorf("image is %.1f MB, over the 5 MB limit", float64(info.Size())/1_000_000)
	}

	if _, _, err := image.DecodeConfig(f); err != nil {
		return "", fmt.Errorf("not a png, jpeg or gif image")
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	file, err := s.client.UploadFile(context.Background(), user, filepath.Base(path), f, info.Size())
	if err != nil {
		return "", err
	}

	return s.client.GetFileURL(file)
}

// cleanPath turns path typed or dropped onto the terminal into a file path, removing quotes, shell escapes and
// file:// prefix, and expanding ~.
func cleanPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("enter a path to an image")
	}

	if n := len(path); n >= 2 && (path[0] == '\'' || path[0] == '"') && path[n-1] == path[0] {
		path = path[1 : n-1]
	} else {
		var b strings.Builder
		escaped := false
		for _, r := range path {
			if r == '\\' && !escaped {
				escaped = true
				continue
			}

			escaped = false
			b.WriteRune(r)
		}

		path = b.String()
	}

	if strings.HasPrefix(path, "file://") {
		u, err := url.Parse(path)
		if err != nil {
			return "", fmt.Errorf("invalid file url: %w", err)
		}

		path = u.Path
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand ~: %w", err)
		}

		path = filepath.Join(home, path[1:])
	}

	return path, nil
}

// saveImage downloads image at url into a temporary file named with its real extension and returns its path.
// Server sends images without content type, so opening their URL makes browsers download a file instead.
func (s *Service) saveImage(url string) (string, error) {
	data, err := s.download(url)
	if err != nil {
		return "", err
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	dir := filepath.Join(os.TempDir(), "friendly-images")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%x.%s", sha256.Sum256([]byte(url)), format))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return path, nil
}
