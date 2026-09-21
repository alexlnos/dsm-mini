package bot

import "net/url"

func decodeQuery(s string) (string, error) {
	return url.QueryUnescape(s)
}
