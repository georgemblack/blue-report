package urltools

import (
	"testing"
)

func TestIsShortenedURL(t *testing.T) {
	shortened := IsShortenedURL("https://wapo.st/abc")
	if !shortened {
		t.Errorf("expected true, got false")
	}

	shortened = IsShortenedURL("https://goo.gl/123")
	if !shortened {
		t.Errorf("expected true, got false")
	}

	shortened = IsShortenedURL("https://www.youtube.com/watch?v=bogus")
	if shortened {
		t.Errorf("expected false, got true")
	}
}
