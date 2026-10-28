// Package supportwire defines authenticated support frames and bounded transport.
package supportwire

// ManagementHealth deliberately excludes inventory, account data and credentials.
type ManagementHealth struct {
	Connector string `json:"connector"`
	Registry  string `json:"registry"`
}
