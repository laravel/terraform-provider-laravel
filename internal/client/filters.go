package client

import "net/url"

// filterQuery turns filters into the "?filter[key]=value" query string the
// API's list routes take. Empty values are left out, and so is the "?" when
// nothing is left.
func filterQuery(filters map[string]string) string {
	q := url.Values{}
	for k, v := range filters {
		if v != "" {
			q.Set("filter["+k+"]", v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}
