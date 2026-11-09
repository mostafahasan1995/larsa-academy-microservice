package enums

type RenewalStatus string

const (
	RenewalStatusActive    RenewalStatus = "active"
	RenewalStatusCancelled RenewalStatus = "cancelled"
)

type RenewalType string

const (
	RenewalTypeMonthly RenewalType = "monthly"
	RenewalTypeYearly  RenewalType = "yearly"
)

type RenewalContentType string

const (
	RenewalContentTypeCourse   RenewalContentType = "course"
	RenewalContentTypeCoaching RenewalContentType = "coaching"
	RenewalContentTypeBundle   RenewalContentType = "bundle"
	RenewalContentTypeMaterial RenewalContentType = "material"
)
