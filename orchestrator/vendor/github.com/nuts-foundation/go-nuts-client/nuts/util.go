package nuts

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

var ErrNutsNodeUnreachable = errors.New("nuts node unreachable")

func UnwrapAPIError(err error) error {
	if _, ok := err.(net.Error); ok {
		return errors.Join(ErrNutsNodeUnreachable, err)
	}
	return err
}

func ParseResponse[T any](err error, httpResponse *http.Response, fn func(rsp *http.Response) (*T, error)) (*T, error) {
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", UnwrapAPIError(err))
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		responseData, _ := io.ReadAll(httpResponse.Body)
		detail := "Response data:\n----------------\n" + strings.TrimSpace(string(responseData)) + "\n----------------"
		return nil, fmt.Errorf("non-OK status code (status=%s, url=%s)\n%s", httpResponse.Status, httpResponse.Request.URL, detail)
	}
	if !strings.Contains(httpResponse.Header.Get("Content-Type"), "application/json") {
		return nil, fmt.Errorf("unexpected response content type: %s", httpResponse.Header.Get("Content-Type"))
	}
	result, err := fn(httpResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}
