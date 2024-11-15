package server

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type QueryParams struct {
	Platform string
	Limit    int
	Days     int
}

const (
	StatusSuccess = "success"
	StatusError   = "error"
)

var ValidPlatforms = map[string]bool{
	"linux-amd64":  true,
	"linux-arm64":  true,
	"darwin-amd64": true,
	"darwin-arm64": true,
}
