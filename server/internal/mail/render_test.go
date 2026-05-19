package mail_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"confero/internal/mail"
)

// update golden files by running: UPDATE_GOLDEN=1 go test ./internal/mail/...
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func fixedTime(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 9, 0, 0, 0, time.UTC)
}

func TestRenderReminder(t *testing.T) {
	data := mail.ReminderData{
		UserName:          "Alice Tester",
		ConferenceName:    "International Conference on Education",
		ConferenceAcronym: "ICEd",
		DeadlineKind:      "submission",
		DeadlineDate:      fixedTime(2026, 9, 15),
		LeadTimeDays:      7,
	}

	gotText, gotHTML, err := mail.RenderReminder(data)
	require.NoError(t, err)

	checkGolden(t, "testdata/reminder.txt.golden", gotText)
	checkGolden(t, "testdata/reminder.html.golden", gotHTML)
}

func TestRenderDigest_WithItems(t *testing.T) {
	data := mail.DigestData{
		UserName:  "Bob Chair",
		WeekStart: fixedTime(2026, 9, 14),
		Items: []mail.DigestItem{
			{
				ConferenceName:    "International Conference on Education",
				ConferenceAcronym: "ICEd",
				DeadlineKind:      "submission",
				DeadlineDate:      fixedTime(2026, 9, 15),
			},
			{
				ConferenceName:    "Workshop on CS Education",
				ConferenceAcronym: "WCSE",
				DeadlineKind:      "abstract",
				DeadlineDate:      fixedTime(2026, 9, 20),
			},
		},
	}

	gotText, gotHTML, err := mail.RenderDigest(data)
	require.NoError(t, err)

	checkGolden(t, "testdata/digest_with_items.txt.golden", gotText)
	checkGolden(t, "testdata/digest_with_items.html.golden", gotHTML)
}

func TestRenderDigest_Empty(t *testing.T) {
	data := mail.DigestData{
		UserName:  "Carol Empty",
		WeekStart: fixedTime(2026, 9, 14),
		Items:     nil,
	}

	gotText, gotHTML, err := mail.RenderDigest(data)
	require.NoError(t, err)

	checkGolden(t, "testdata/digest_empty.txt.golden", gotText)
	checkGolden(t, "testdata/digest_empty.html.golden", gotHTML)
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	if updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		t.Logf("updated golden file %s", path)
		return
	}
	golden, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("golden file %s does not exist; run with UPDATE_GOLDEN=1 to create it", path)
	}
	require.NoError(t, err)
	assert.Equal(t, string(golden), got, "output does not match golden file %s", path)
}
