package urltools

import "testing"

func TestCleanWithInvalidURL(t *testing.T) {
	result := Clean("invalid")
	if result != "invalid" {
		t.Errorf("expected 'invalid', got '%s'", result)
	}
}

func TestCleanWithAllowedQueryParams(t *testing.T) {
	result := Clean("https://abcnews.go.com/page?story=id&id=12345678&something=else")
	expected := "https://abcnews.go.com/page?id=12345678"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}

	result = Clean("https://www.youtube.com/watch?v=OpViD7KxK-I&feature=web")
	expected = "https://www.youtube.com/watch?v=OpViD7KxK-I"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}

	result = Clean("https://m.youtube.com/watch?v=OpViD7KxK-I&feature=web")
	expected = "https://www.youtube.com/watch?v=OpViD7KxK-I"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}

	result = Clean("https://theblue.report/page?bogus=bogus")
	expected = "https://theblue.report/page"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestCleanWithYouTubeTransformation(t *testing.T) {
	result := Clean("https://youtu.be/OpViD7KxK-I?bogus=bogus")
	expected := "https://www.youtube.com/watch?v=OpViD7KxK-I"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestCleanWithYouTubeMobileTransformation(t *testing.T) {
	result := Clean("https://m.youtube.com/watch?v=frPvUIchc9s")
	expected := "https://www.youtube.com/watch?v=frPvUIchc9s"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestCleanSubstackWithOpenLink(t *testing.T) {
	result := Clean("https://open.substack.com/pub/verdeallday/p/ilie-sanchez-new-austin-fc-signing-analysis?r=46a2f&utm_campaign=post&utm_medium=web&showWelcomeOnShare=true")
	expected := "https://verdeallday.substack.com/p/ilie-sanchez-new-austin-fc-signing-analysis"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestCleanSubstackWithoutOpenLink(t *testing.T) {
	result := Clean("https://newsletter.pragmaticengineer.com/p/state-of-eng-market-2024?r=46a2f&utm_campaign=post&utm_medium=web&showWelcomeOnShare=true")
	expected := "https://newsletter.pragmaticengineer.com/p/state-of-eng-market-2024"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestCleanWithFragment(t *testing.T) {
	result := Clean("https://theblue.report/some-page#fragment")
	expected := "https://theblue.report/some-page"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}
