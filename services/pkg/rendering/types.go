package rendering

type BrowserRequest struct {
	URL      string                   `json:"url"`
	Elements []BrowserRequestElements `json:"elements"`
}

type BrowserRequestElements struct {
	Selector string `json:"selector"`
}

type BrowserResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		Results []struct {
			Attributes []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"attributes"`
			Text string `json:"text"`
		} `json:"results"`
		Selector string `json:"selector"`
	} `json:"result"`
}

// Get a list of attributes for a given selector.
// For example, if a user requestsed all 'href' attributes for 'a' tags, the resulting list would contain all 'href' attributes.
// '<a href="bogus.com"></a><a href="bogus2.com"></a>' would return ["bogus.com", "bogus2.com"]
func (r BrowserResponse) GetAttribute(selector, attribute string) []string {
	attributes := []string{}

	for _, result := range r.Result {
		if result.Selector == selector {
			for _, element := range result.Results {
				for _, attr := range element.Attributes {
					if attr.Name == attribute {
						attributes = append(attributes, attr.Value)
					}
				}
			}
		}
	}

	return attributes
}
