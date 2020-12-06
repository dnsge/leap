package client

import "net/url"

type Config struct {
	Domain    string
	Subdomain string
	LocalPort int
	Secure    bool
}

func (cfg *Config) getURL(scheme, path string) string {
	u := url.URL{
		Scheme: scheme,
		Host:   cfg.Domain,
		Path:   path,
	}
	return u.String()
}

func (cfg *Config) httpScheme() string {
	if cfg.Secure {
		return "https"
	} else {
		return "http"
	}
}

func (cfg *Config) wsScheme() string {
	if cfg.Secure {
		return "wss"
	} else {
		return "ws"
	}
}

func (cfg *Config) getHttpURL(path string) string {

	return cfg.getURL(cfg.httpScheme(), path)
}

func (cfg *Config) getWsURL(path string) string {
	return cfg.getURL(cfg.wsScheme(), path)
}
