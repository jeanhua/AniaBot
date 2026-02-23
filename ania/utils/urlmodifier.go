package utils

import "net/url"

type URLModifier struct {
	url *url.URL
}

func NewURLModifier(rawURL string) (*URLModifier, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &URLModifier{url: u}, nil
}

func (m *URLModifier) SetQuery(key, value string) *URLModifier {
	q := m.url.Query()
	q.Set(key, value)
	m.url.RawQuery = q.Encode()
	return m
}

func (m *URLModifier) DelQuery(key string) *URLModifier {
	q := m.url.Query()
	q.Del(key)
	m.url.RawQuery = q.Encode()
	return m
}

func (m *URLModifier) String() string {
	return m.url.String()
}
