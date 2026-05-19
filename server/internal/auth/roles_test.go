package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"confero/internal/auth"
)

func TestDecodeRoles_Member(t *testing.T) {
	r := auth.DecodeRoles([]string{"cs-edu-chair"}, "cs-edu-chair", "cs-edu-chair-admin")
	require.True(t, r.Member)
	require.False(t, r.Admin)
	require.Equal(t, []string{"member"}, r.ToStringSlice())
}

func TestDecodeRoles_Admin(t *testing.T) {
	r := auth.DecodeRoles([]string{"cs-edu-chair-admin"}, "cs-edu-chair", "cs-edu-chair-admin")
	require.True(t, r.Member, "admin implies member")
	require.True(t, r.Admin)
	require.Equal(t, []string{"member", "admin"}, r.ToStringSlice())
}

func TestDecodeRoles_Both(t *testing.T) {
	r := auth.DecodeRoles(
		[]string{"cs-edu-chair", "cs-edu-chair-admin"},
		"cs-edu-chair", "cs-edu-chair-admin",
	)
	require.True(t, r.Member)
	require.True(t, r.Admin)
}

func TestDecodeRoles_Neither(t *testing.T) {
	r := auth.DecodeRoles([]string{"some-other-group"}, "cs-edu-chair", "cs-edu-chair-admin")
	require.False(t, r.Member)
	require.False(t, r.Admin)
	require.Nil(t, r.ToStringSlice())
}

func TestDecodeRoles_Empty(t *testing.T) {
	r := auth.DecodeRoles(nil, "cs-edu-chair", "cs-edu-chair-admin")
	require.False(t, r.Member)
	require.False(t, r.Admin)
}
