package urltools

import (
	"testing"
)

func TestIsShortenedURL(t *testing.T) {
	shortened := IsShortened("https://wapo.st/abc")
	if !shortened {
		t.Errorf("expected true, got false")
	}

	shortened = IsShortened("https://goo.gl/123")
	if !shortened {
		t.Errorf("expected true, got false")
	}

	shortened = IsShortened("https://www.youtube.com/watch?v=bogus")
	if shortened {
		t.Errorf("expected false, got true")
	}
}
