package community

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

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

	size, _, err := inspect(f)
	if err != nil {
		return "", err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	file, err := s.client.UploadFile(context.Background(), user, filepath.Base(path), f, size)
	if err != nil {
		return "", err
	}

	return s.client.GetFileURL(file)
}

// uploadClipboard sends the image in the system clipboard and returns its URL for embedding into a post.
func (s *Service) uploadClipboard(user *sdk.Authorization) (string, error) {
	path, err := clipboardImage()
	if err != nil {
		return "", err
	}
	defer os.Remove(path) //nolint:errcheck

	return s.upload(user, path)
}

// inspect checks that f is a png, jpeg or gif within maxUploadBytes and returns its size and dimensions. It reads the
// start of f.
func inspect(f *os.File) (int64, image.Config, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, image.Config{}, fmt.Errorf("failed to read image: %w", err)
	}

	if info.Size() > maxUploadBytes {
		return 0, image.Config{}, fmt.Errorf("image is %.1f MB, over the 5 MB limit", float64(info.Size())/1_000_000)
	}

	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, image.Config{}, fmt.Errorf("not a png, jpeg or gif image")
	}

	return info.Size(), config, nil
}

// describe returns a line about the image at path typed into the attach prompt, like photo.png · 1.2 MB · 800x600.
// It returns "" while path isn't a file, and the reason when the file can't be attached.
func describe(path string) (string, error) {
	path, err := cleanPath(path)
	if err != nil {
		return "", nil
	}

	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close() //nolint:errcheck

	size, config, err := inspect(f)
	if err != nil {
		return "", err
	}

	weight := fmt.Sprintf("%d KB", max(size/1000, 1))
	if size >= 1_000_000 {
		weight = fmt.Sprintf("%.1f MB", float64(size)/1_000_000)
	}

	return fmt.Sprintf("%s · %s · %dx%d", filepath.Base(path), weight, config.Width, config.Height), nil
}

// clipboardImage saves the image in the system clipboard to a temporary png file and returns its path. It reads the
// clipboard with osascript on macOS, and with wl-paste or xclip elsewhere.
func clipboardImage() (string, error) {
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "darwin":
		cmd = exec.Command("osascript", "-e", "the clipboard as «class PNGf»")
	case os.Getenv("WAYLAND_DISPLAY") != "":
		cmd = exec.Command("wl-paste", "--type", "image/png")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-target", "image/png", "-out")
	}

	data, err := cmd.Output()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		return "", fmt.Errorf("no image in the clipboard")
	case err != nil:
		return "", fmt.Errorf("failed to read the clipboard: %w", err)
	}

	// osascript prints the image as «data PNGf89504E47...» in hex
	if runtime.GOOS == "darwin" {
		hexed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(data)), "«data PNGf"), "»")
		if data, err = hex.DecodeString(hexed); err != nil {
			return "", fmt.Errorf("failed to read the clipboard image: %w", err)
		}
	}

	f, err := os.CreateTemp("", "clipboard-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to save the clipboard image: %w", err)
	}

	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		os.Remove(f.Name()) //nolint:errcheck
		return "", fmt.Errorf("failed to save the clipboard image: %w", err)
	}

	return f.Name(), nil
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

// imageExts are extensions of files the path prompt completes.
var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true}

// completions returns the entries in the directory of typed path whose names start with its last part, ignoring case.
// It lists directories with a trailing slash and png, jpeg or gif files, sorted by name. Hidden entries show once the
// last part starts with a dot.
func completions(typed string) []string {
	i := strings.LastIndex(typed, "/") + 1
	prefix := strings.ToLower(strings.ReplaceAll(typed[i:], `\`, ""))
	dir := "."
	if i > 0 {
		var err error
		if dir, err = cleanPath(typed[:i]); err != nil {
			return nil
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), prefix) || strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		isDir := entry.IsDir()
		// a symlink counts as what it points to
		if entry.Type()&fs.ModeSymlink != 0 {
			info, err := os.Stat(filepath.Join(dir, name))
			isDir = err == nil && info.IsDir()
		}

		switch {
		case isDir:
			found = append(found, name+"/")
		case imageExts[strings.ToLower(filepath.Ext(name))]:
			found = append(found, name)
		}
	}

	slices.SortFunc(found, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})

	return found
}

// droppedImages returns the paths of image files dropped onto the terminal, or nil when text is anything else.
// Terminals paste dropped files as absolute paths separated by spaces or new lines, with spaces inside them escaped
// or quoted. A single path with bare spaces counts too.
func droppedImages(text string) []string {
	for _, paths := range [][]string{{text}, splitPaths(text)} {
		if len(paths) > 0 && !slices.ContainsFunc(paths, func(path string) bool { return !isImageFile(path) }) {
			return paths
		}
	}

	return nil
}

// splitPaths splits text at spaces outside quotes and escapes. The paths keep their quotes and escapes for cleanPath.
func splitPaths(text string) []string {
	var paths []string
	var path strings.Builder
	var quote rune
	escaped := false
	for _, r := range text {
		switch {
		case escaped:
			escaped = false
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\\':
			escaped = true
		case r == '\'' || r == '"':
			quote = r
		case unicode.IsSpace(r):
			if path.Len() > 0 {
				paths = append(paths, path.String())
				path.Reset()
			}

			continue
		}

		path.WriteRune(r)
	}

	if path.Len() > 0 {
		paths = append(paths, path.String())
	}

	return paths
}

// isImageFile reports whether text is an absolute path to a file with a png, jpeg or gif extension.
func isImageFile(text string) bool {
	path, err := cleanPath(text)
	if err != nil || !filepath.IsAbs(path) || !imageExts[strings.ToLower(filepath.Ext(path))] {
		return false
	}

	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
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
