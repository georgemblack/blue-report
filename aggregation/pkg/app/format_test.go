package app

import (
	"testing"
)

// This test uses actual examples taken from the website.
func TestFormatPost(t *testing.T) {
	post := "Hello, world!"
	expected := "Hello, world!"
	actual := formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "Jasmine Crockett Has Fiery Warning For 'Broke' Red States: 'We're In The Find Out Phase \n#FAFO \n#RedStates \n\nwww.comicsands.com/crockett-bro..."
	expected = "Jasmine Crockett Has Fiery Warning For 'Broke' Red States: 'We're In The Find Out Phase #FAFO #RedStates"
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "This x10000000000\n\n“In other words: We, the opposition, are the majority. Take heart.”\n\nwww.hamiltonnolan.com/p/they-are-a..."
	expected = "This x10000000000 “In other words: We, the opposition, are the majority. Take heart.”"
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "We are in a dangerous time but here are two important facts:\n\n1. Trump's base of support is a minority.\n2. That base is going to shrink as his agenda is enacted.\n\nThe opposition is the majority. No moping allowed. \n\nwww.hamiltonnolan.com/p/they-are-a..."
	expected = "We are in a dangerous time but here are two important facts: 1. Trump's base of support is a minority. 2. That base is going to shrink as his agenda is enacted. The opposition is the majority. No moping allowed."
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "Hegseth's comments about General Brown having his position (probably) because of his skin color are reprehensible, but not at all surprising.\n\napnews.com/article/trum..."
	expected = "Hegseth's comments about General Brown having his position (probably) because of his skin color are reprehensible, but not at all surprising."
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "NEW: Get ready for delayed tax refunds, long hold times + dropped calls with the IRS, thanks to Trump illegally firing more than 6,000 employees there today. \n\n*Except if you're a wealthy tax evader. You will now \"feast\" as the IRS strains to function. www.huffpost.com/entry/irs-ma..."
	expected = "Get ready for delayed tax refunds, long hold times + dropped calls with the IRS, thanks to Trump illegally firing more than 6,000 employees there today. *Except if you're a wealthy tax evader. You will now \"feast\" as the IRS strains to function."
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "A must read! @marcelias.bsky.social is a real Mench!\nwww.democracydocket.com/opinion/my-o..."
	expected = "A must read! @marcelias.bsky.social is a real Mench!"
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "🚨BREAKING: In 4-3 decision, Wisconsin Supreme Court reverses lower court decision and DISMISSES voter suppression lawsuit. \n\nA big victory for our clients the WI Alliance for Retired Americans and voting rights in Wisconsin. Another stinging loss for the GOP! www.democracydocket.com/cases/wiscon..."
	expected = "In 4-3 decision, Wisconsin Supreme Court reverses lower court decision and DISMISSES voter suppression lawsuit. A big victory for our clients the WI Alliance for Retired Americans and voting rights in Wisconsin. Another stinging loss for the GOP!"
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "If you’re not confirmed by the Senate then you have no business meddling in the lives of MILLIONS of Americans\n\nWe are suing DOGE for violating the constitution & illegally seizing power\n\nI’ll discuss tonight @ 8PM ET on @insidewithpsaki.msnbc.com 👇\nstatedemocracydefenders.org/fund/new-law..."
	expected = "If you’re not confirmed by the Senate then you have no business meddling in the lives of MILLIONS of Americans We are suing DOGE for violating the constitution & illegally seizing power I’ll discuss tonight @ 8PM ET on @insidewithpsaki.msnbc.com 👇"
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}

	post = "🔴 Something something!"
	expected = "Something something!"
	actual = formatPost(post)
	if actual != expected {
		t.Errorf("expected '%s', got '%s'", expected, actual)
	}
}
