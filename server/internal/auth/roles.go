package auth

// Roles represents a user's role set.
type Roles struct {
	Member bool
	Admin  bool
}

// DecodeRoles inspects the OIDC claim groups and maps them to Roles.
// Admin implies Member (FR-22b).
func DecodeRoles(groups []string, memberValue, adminValue string) Roles {
	r := Roles{}
	for _, g := range groups {
		if g == memberValue {
			r.Member = true
		}
		if g == adminValue {
			r.Member = true
			r.Admin = true
		}
	}
	return r
}

// ToStringSlice returns roles as the canonical string slice used in JWT claims.
func (r Roles) ToStringSlice() []string {
	if r.Admin {
		return []string{"member", "admin"}
	}
	if r.Member {
		return []string{"member"}
	}
	return nil
}
