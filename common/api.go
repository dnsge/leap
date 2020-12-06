package common

type SubdomainRequest struct {
	Subdomain string `json:"subdomain"`
}

type TokenResponse struct {
	Subdomain string `json:"subdomain"`
	Token     string `json:"token"`
}
