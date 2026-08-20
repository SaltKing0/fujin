package statuspage

import "time"

type StatusIndicator string

const (
	StatusNone     StatusIndicator = "none"
	StatusMinor    StatusIndicator = "minor"
	StatusMajor    StatusIndicator = "major"
	StatusCritical StatusIndicator = "critical"
)

type StatusInfo struct {
	Indicator   StatusIndicator `json:"indicator"`
	Description string          `json:"description"`
}

type PageInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updated_at"`
}

type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type Incident struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Impact     string     `json:"impact"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Shortlink  string     `json:"shortlink"`
}

type SummaryResponse struct {
	Page       PageInfo    `json:"page"`
	Components []Component `json:"components"`
	Status     StatusInfo  `json:"status"`
}
