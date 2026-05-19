package service

import (
	"testing"
	"time"

	"confero/internal/api"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func inputWith(mutate func(*api.ConferenceInput)) api.ConferenceInput {
	inp := api.ConferenceInput{
		Name:     "SIGCSE",
		Acronym:  "SIGCSE",
		Year:     2027,
		Location: "Portland, OR",
	}
	if mutate != nil {
		mutate(&inp)
	}
	return inp
}

func timep(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

func datep(s string) *openapi_types.Date {
	t, _ := time.Parse("2006-01-02", s)
	return &openapi_types.Date{Time: t}
}

func TestValidateInput_HappyPath(t *testing.T) {
	if err := validateInput(inputWith(nil)); err != nil {
		t.Fatalf("unexpected error for valid input: %v", err)
	}
}

func TestValidateInput_YearRange(t *testing.T) {
	cases := []struct {
		year    int
		wantErr bool
	}{
		{1999, true},
		{2000, false},
		{2100, false},
		{2101, true},
	}
	for _, tc := range cases {
		inp := inputWith(func(i *api.ConferenceInput) { i.Year = tc.year })
		err := validateInput(inp)
		if tc.wantErr && err == nil {
			t.Errorf("year %d: expected error", tc.year)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("year %d: unexpected error: %v", tc.year, err)
		}
	}
}

func TestValidateInput_BlankName(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) { i.Name = "   " })
	err := validateInput(inp)
	if err == nil {
		t.Fatal("expected error for blank name")
	}
}

func TestValidateInput_BlankAcronym(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) { i.Acronym = "" })
	if err := validateInput(inp); err == nil {
		t.Fatal("expected error for blank acronym")
	}
}

func TestValidateInput_DeadlineOrder_AbstractAfterPrimary(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) {
		i.AbstractDeadline = timep("2027-06-15T00:00:00Z")
		i.PrimaryDeadline = timep("2027-06-01T00:00:00Z") // primary before abstract
	})
	if err := validateInput(inp); err == nil {
		t.Fatal("expected error: abstract after primary")
	}
}

func TestValidateInput_DeadlineOrder_Valid(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) {
		i.AbstractDeadline = timep("2027-05-15T00:00:00Z")
		i.PrimaryDeadline = timep("2027-06-01T00:00:00Z")
		i.NotificationDate = timep("2027-07-01T00:00:00Z")
		i.CameraReadyDate = timep("2027-08-01T00:00:00Z")
	})
	if err := validateInput(inp); err != nil {
		t.Fatalf("unexpected error for valid deadlines: %v", err)
	}
}

func TestValidateInput_DeadlineOrder_PrimaryAfterNotification(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) {
		i.PrimaryDeadline = timep("2027-08-01T00:00:00Z")
		i.NotificationDate = timep("2027-07-01T00:00:00Z") // notification before primary
	})
	if err := validateInput(inp); err == nil {
		t.Fatal("expected error: primary after notification")
	}
}

func TestValidateInput_EventDates_StartAfterEnd(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) {
		i.EventStartDate = datep("2027-10-15")
		i.EventEndDate = datep("2027-10-10") // end before start
	})
	if err := validateInput(inp); err == nil {
		t.Fatal("expected error: start after end")
	}
}

func TestValidateInput_EventDates_SameDay(t *testing.T) {
	inp := inputWith(func(i *api.ConferenceInput) {
		i.EventStartDate = datep("2027-10-10")
		i.EventEndDate = datep("2027-10-10")
	})
	if err := validateInput(inp); err != nil {
		t.Fatalf("unexpected error for same-day event: %v", err)
	}
}

func TestValidateInput_AcceptanceRate_OutOfRange(t *testing.T) {
	f := float32(101.0)
	inp := inputWith(func(i *api.ConferenceInput) { i.AcceptanceRatePct = &f })
	if err := validateInput(inp); err == nil {
		t.Fatal("expected error for acceptance rate > 100")
	}
}

func TestValidateInput_H5Index_Negative(t *testing.T) {
	n := -1
	inp := inputWith(func(i *api.ConferenceInput) { i.H5Index = &n })
	if err := validateInput(inp); err == nil {
		t.Fatal("expected error for negative h5_index")
	}
}

func TestValidateInput_NullableDeadlines_NoError(t *testing.T) {
	// Only primary set, no abstract — valid (no ordering constraint to check)
	inp := inputWith(func(i *api.ConferenceInput) {
		i.PrimaryDeadline = timep("2027-06-01T00:00:00Z")
	})
	if err := validateInput(inp); err != nil {
		t.Fatalf("unexpected error when abstract deadline is nil: %v", err)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"CS-Education", "cs-education"},
		{"  Hello World  ", "hello-world"},
		{"foo--bar", "foo-bar"},
		{"A*B*C", "a-b-c"},
		{"", ""},
	}
	for _, tc := range cases {
		got := Slugify(tc.in)
		if got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
