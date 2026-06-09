package status

import "encoding/json"

type Status struct {
	VPN     VPNStatus     `json:"vpn"`
	Gateway GatewayStatus `json:"gateway"`
	Bypass  BypassStatus  `json:"bypass"`
	Lock    LockStatus    `json:"lock"`
	Config  ConfigStatus  `json:"config"`
	Errors  []string      `json:"errors"`
}

type VPNStatus struct {
	Name   string   `json:"name"`
	Types  []string `json:"types"`
	Status string   `json:"status"`
}

type GatewayStatus struct {
	Via   string `json:"via"`
	Dev   string `json:"dev"`
	Error string `json:"error"`
}

type BypassStatus struct {
	DomainCount int    `json:"domain_count"`
	IPCount     int    `json:"ip_count"`
	RouteCount  int    `json:"route_count"`
	StatePath   string `json:"state_path"`
}

type LockStatus struct {
	DomainCount int    `json:"domain_count"`
	NFTActive   bool   `json:"nft_active"`
	HostsBlock  bool   `json:"hosts_block"`
	Error       string `json:"error"`
}

type ConfigStatus struct {
	Path string `json:"path"`
}

func Parse(data []byte) (Status, error) {
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return Status{}, err
	}
	return status, nil
}
