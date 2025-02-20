package urltools

import "testing"

func TestIgnore(t *testing.T) {
	ignore := Ignore("invalid")
	if !ignore {
		t.Errorf("expected true, got false")
	}

	ignore = Ignore("https://www.nytimes.com/2025/02/18/us/politics/fda-food-safety-jim-jones-resignation.html")
	if ignore {
		t.Errorf("expected false, got true")
	}

	ignore = Ignore("https://www.kxan.com/weather/forecast/todays-forecast/")
	if ignore {
		t.Errorf("expected false, got true")
	}
}
