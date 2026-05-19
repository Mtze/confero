// Package importer provides pluggable conference import from structured formats.
package importer

import (
	"context"
	"io"
)

// Result summarises a completed import operation.
type Result struct {
	Created int
	Updated int
	Skipped int
	Errors  []string
}

// ConferenceInput is the normalised representation shared by all importers.
type ConferenceInput struct {
	Name              string   `yaml:"name"`
	Acronym           string   `yaml:"acronym"`
	Year              int32    `yaml:"year"`
	Location          string   `yaml:"location"`
	WebsiteURL        string   `yaml:"website_url"`
	CFPURL            string   `yaml:"cfp_url"`
	PrimaryDeadline   string   `yaml:"primary_deadline"`
	AbstractDeadline  string   `yaml:"abstract_deadline"`
	NotificationDate  string   `yaml:"notification_date"`
	CameraReadyDate   string   `yaml:"camera_ready_date"`
	EventStartDate    string   `yaml:"event_start_date"`
	EventEndDate      string   `yaml:"event_end_date"`
	CoreRank          string   `yaml:"core_rank"`
	H5Index           int32    `yaml:"h5_index"`
	AcceptanceRatePct float64  `yaml:"acceptance_rate_pct"`
	DblpKey           string   `yaml:"dblp_key"`
	Notes             string   `yaml:"notes"`
	Tags              []string `yaml:"tags"`
	Tracks            []string `yaml:"tracks"`
}

// Importer parses a source and returns a list of ConferenceInputs.
type Importer interface {
	Parse(r io.Reader) ([]ConferenceInput, error)
}

// Validate returns an error string if the input has any required fields missing.
func (c *ConferenceInput) Validate() string {
	if c.Name == "" {
		return "name is required"
	}
	if c.Acronym == "" {
		return "acronym is required"
	}
	if c.Year < 2000 || c.Year > 2100 {
		return "year must be between 2000 and 2100"
	}
	return ""
}

// Doer executes the upsert of a single validated ConferenceInput.
// It returns (created bool, error).
type Doer interface {
	UpsertFromImport(ctx context.Context, in ConferenceInput, actorID string) (created bool, err error)
}
