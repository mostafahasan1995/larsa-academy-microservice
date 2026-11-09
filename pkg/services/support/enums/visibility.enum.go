package enums

type BenefitVisibility string

const (
	VisibilityPublic  BenefitVisibility = "public"
	VisibilityPrivate BenefitVisibility = "private"
)

// IsValid checks if the visibility value is valid
func (v BenefitVisibility) IsValid() bool {
	switch v {
	case VisibilityPublic, VisibilityPrivate:
		return true
	}
	return false
}
