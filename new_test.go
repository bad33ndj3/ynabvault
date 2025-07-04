package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type errorBody struct{ io.Reader }

func (e *errorBody) Close() error { return errors.New("close error") }

type staticRoundTripper struct{}

func (staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &errorBody{Reader: strings.NewReader("data")},
	}, nil
}

func TestHttpGetCloseError(t *testing.T) {
	client := &http.Client{Transport: staticRoundTripper{}}
	_, err := httpGet(client, "http://example", "tok")
	if err == nil || !strings.Contains(err.Error(), "close error") {
		t.Fatalf("expected close error, got %v", err)
	}
}
