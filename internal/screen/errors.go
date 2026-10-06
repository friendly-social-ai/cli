package screen

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	sdk "github.com/friendly-social/golang-sdk"
)

// ErrorText returns a short message about err for the status, and logs err in full.
func ErrorText(err error) string {
	log.Printf("error: %v", err)

	var apiErr sdk.APIError
	var netErr net.Error
	switch {
	case errors.As(err, &apiErr):
		switch code := apiErr.Code; {
		case code == http.StatusUnauthorized:
			return "session expired, log in again"
		case code == http.StatusForbidden:
			return "not allowed"
		case code == http.StatusNotFound:
			return "not found"
		case code == http.StatusTooManyRequests:
			return "too many requests, try again later"
		case code >= http.StatusInternalServerError:
			return "server error, try again later"
		default:
			return fmt.Sprintf("request failed (%d)", code)
		}
	case errors.As(err, &netErr):
		return "can't reach the server"
	}

	// the innermost error is the specific one, like an invalid e-mail
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}

	return err.Error()
}
