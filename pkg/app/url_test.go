package app_test

import (
	"testing"

	"github.com/georgemblack/blue-report/pkg/app"
)

func TestNormalizeYouTube(t *testing.T) {
	// Mobile URL with multiple query params
	dirty := "https://m.youtube.com/watch?v=5evhacpji5s&feature=shared"
	clean := "https://youtu.be/5evhacpji5s"
	result := app.Normalize(dirty)
	if result != clean {
		t.Errorf("expected '%s', got '%s'", clean, result)
	}

	// 'www' URL with multiple query params
	dirty = "https://www.youtube.com/watch?v=5evhacpji5s&feature=shared"
	clean = "https://youtu.be/5evhacpji5s"
	result = app.Normalize(dirty)
	if result != clean {
		t.Errorf("expected '%s', got '%s'", clean, result)
	}

	// Standard URL with one query param
	dirty = "https://youtube.com/watch?v=5evhacpji5s&utm_source=example"
	clean = "https://youtu.be/5evhacpji5s"
	result = app.Normalize(dirty)
	if result != clean {
		t.Errorf("expected '%s', got '%s'", clean, result)
	}
}

func TestNormalizeRemoveQueryParams(t *testing.T) {
	dirty := "https://example.com/page?foo=bar&baz=qux"
	clean := "https://example.com/page"
	result := app.Normalize(dirty)
	if result != clean {
		t.Errorf("expected '%s', got '%s'", clean, result)
	}
}

func TestNormalizeQueryParamAllowList(t *testing.T) {
	dirty := "https://abcnews.go.com/page?story=12345678"
	clean := "https://abcnews.go.com/page?story=12345678"
	result := app.Normalize(dirty)
	if result != clean {
		t.Errorf("expected '%s', got '%s'", clean, result)
	}
}
