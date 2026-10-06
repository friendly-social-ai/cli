package auth

import "net/http"

// ExpiredMsg reports that the server rejected the saved session.
type ExpiredMsg struct{}

// Transport is http.RoundTripper that calls Expired when the server rejects the token of a logged in request. Login
// requests carry no token, so their failures don't count.
type Transport struct {
	Expired func()
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err == nil && resp.StatusCode == http.StatusUnauthorized && req.Header.Get("X-Token") != "" {
		t.Expired()
	}

	return resp, err
}
