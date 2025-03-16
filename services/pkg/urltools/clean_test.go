package urltools

import "testing"

type CleanTest struct {
	Input    string
	Expected string
}

func TestCleanWithInvalidURL(t *testing.T) {
	result := Clean("invalid")
	if result != "invalid" {
		t.Errorf("expected 'invalid', got '%s'", result)
	}
}

func TestCleanWithAllowedQueryParams(t *testing.T) {
	tests := []CleanTest{
		{
			Input:    "https://abcnews.go.com/page?story=id&id=12345678&something=else",
			Expected: "https://abcnews.go.com/page?id=12345678",
		},
		{
			Input:    "https://www.youtube.com/watch?v=OpViD7KxK-I&feature=web",
			Expected: "https://www.youtube.com/watch?v=OpViD7KxK-I",
		},
		{
			Input:    "https://m.youtube.com/watch?v=OpViD7KxK-I&feature=web",
			Expected: "https://www.youtube.com/watch?v=OpViD7KxK-I",
		},
		{
			Input:    "https://theblue.report/page?bogus=bogus",
			Expected: "https://theblue.report/page",
		},
		{
			Input:    "https://commons.stmarytx.edu/cgi/viewcontent.cgi?article=1051&context=lmej",
			Expected: "https://commons.stmarytx.edu/cgi/viewcontent.cgi?article=1051&context=lmej",
		},
	}

	for _, test := range tests {
		result := Clean(test.Input)
		if result != test.Expected {
			t.Errorf("expected '%s', got '%s'", test.Expected, result)
		}
	}
}

func TestCleanWithYouTubeTransformation(t *testing.T) {
	tests := []CleanTest{
		{
			Input:    "https://youtu.be/OpViD7KxK-I?bogus=bogus",
			Expected: "https://www.youtube.com/watch?v=OpViD7KxK-I",
		},
		{
			Input:    "https://youtube.com/watch?v=au-IVW4M0Oo&si=x6a8c4tGayR1rFqu",
			Expected: "https://www.youtube.com/watch?v=au-IVW4M0Oo",
		},
		{
			Input:    "https://youtube.com/watch?v=3r_SVM4Ui74&feature=shared",
			Expected: "https://www.youtube.com/watch?v=3r_SVM4Ui74",
		},
		{
			Input:    "https://youtube.com/watch?v=Cv2UdJhXdQg",
			Expected: "https://www.youtube.com/watch?v=Cv2UdJhXdQg",
		},
		{
			Input:    "https://youtube.com/watch?v=pUxqVgq_HDQ",
			Expected: "https://www.youtube.com/watch?v=pUxqVgq_HDQ",
		},
		{
			Input:    "https://youtube.com/watch?v=YkJKBK5j2fM",
			Expected: "https://www.youtube.com/watch?v=YkJKBK5j2fM",
		},
	}

	for _, test := range tests {
		result := Clean(test.Input)
		if result != test.Expected {
			t.Errorf("expected '%s', got '%s'", test.Expected, result)
		}
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
