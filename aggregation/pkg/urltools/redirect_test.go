package urltools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test non-redirect status code
func TestFindRedirectWithStandardStatusCode(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Initial request
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/final-destination")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ms.Close()

	result := FindRedirect(ms.URL)
	expected := ""
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// Test temporary redirect status code
func TestFindRedirectStatusCodes(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Initial request
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/final-destination")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}

		// Redirected request
		if r.URL.Path == "/final-destination" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ms.Close()

	result := FindRedirect(ms.URL)
	expected := fmt.Sprintf("%s/final-destination", ms.URL)
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// Ignore any URL that has more than one redirect (i.e. scummy links)
func TestFindRedirectDoubleRedirect(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Initial request
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/final-destination")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}

		// Redirected request
		if r.URL.Path == "/final-destination" {
			w.Header().Set("Location", "/gotcha")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}
	}))
	defer ms.Close()

	result := FindRedirect(ms.URL)
	expected := ""
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// Test absolute URLs (i.e. 'https://theblue.report')
func TestFindRedirectAbsoluteURL(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Initial request
		if r.URL.Path == "/" {
			w.Header().Set("Location", "https://theblue.report")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}
	}))
	defer ms.Close()

	result := FindRedirect(ms.URL)
	expected := "https://theblue.report"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}
