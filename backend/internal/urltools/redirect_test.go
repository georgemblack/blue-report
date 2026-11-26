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
func TestFindRedirectWithStatusCodes(t *testing.T) {
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

// Test two redirects
func TestFindRedirectWithDoubleRedirect(t *testing.T) {
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
	expected := fmt.Sprintf("%s/gotcha", ms.URL)
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// Test two with a relative path
func TestFindRedirectWithRelativePath(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Initial request
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/final-destination")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}

		// Redirected request
		if r.URL.Path == "/final-destination" {
			w.Header().Set("Location", "gotcha")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}
	}))
	defer ms.Close()

	result := FindRedirect(ms.URL)
	expected := fmt.Sprintf("%s/final-destination/gotcha", ms.URL)
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

// Test honeypot services (i.e. 'tollbit.')
func TestFindRedirectHoneypot(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Initial request
		if r.URL.Path == "/" {
			w.Header().Set("Location", "https://tollbit.theblue.report")
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

func TestResolveLocation(t *testing.T) {
	tests := []struct {
		originalURL string
		location    string
		expected    string
	}{
		{
			originalURL: "https://theblue.report",
			location:    "/",
			expected:    "https://theblue.report/",
		},
		{
			originalURL: "https://theblue.report/",
			location:    "/",
			expected:    "https://theblue.report/",
		},
		{
			originalURL: "https://theblue.report/",
			location:    "https://www.theblue.report/",
			expected:    "https://www.theblue.report/",
		},
		{
			originalURL: "https://theblue.report/something",
			location:    "/something",
			expected:    "https://theblue.report/something",
		},
		{
			originalURL: "https://theblue.report/something",
			location:    "bogus",
			expected:    "https://theblue.report/something/bogus",
		},
		{
			originalURL: "https://theblue.report/something/",
			location:    "bogus",
			expected:    "https://theblue.report/something/bogus",
		},
		{
			originalURL: "https://theblue.report/",
			location:    "/bogus",
			expected:    "https://theblue.report/bogus",
		},
		{
			originalURL: "https://theblue.report/",
			location:    "#something",
			expected:    "https://theblue.report/#something",
		},
		{
			originalURL: "https://www.ladders.com/there-is-no-ladder",
			location:    "/there-is-no-ladder/",
			expected:    "https://www.ladders.com/there-is-no-ladder/",
		},
		{
			originalURL: "https://healthmap.org",
			location:    "en",
			expected:    "https://healthmap.org/en",
		},
		{
			originalURL: "https://healthmap.org/en",
			location:    "https://healthmap.org/en/",
			expected:    "https://healthmap.org/en/",
		},
	}

	for _, test := range tests {
		result := resolveLocation(test.originalURL, test.location)
		if result != test.expected {
			t.Errorf("original: '%s', location: '%s', expected: '%s', got: '%s'", test.originalURL, test.location, test.expected, result)
		}
	}
}

func TestFindRedirectWithExcludedHost(t *testing.T) {
	result := FindRedirect("https://www.nature.com/some/article")
	expected := ""
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}
