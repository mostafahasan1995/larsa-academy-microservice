package enums

// RewardType defines the type of reward
type RewardType string

const (
	RewardTypePhysical         RewardType = "Physical"         // Physical gift (هدية فيزيائية)
	RewardTypeCoupon           RewardType = "Coupon"           // Coupon (قسيمة)
	RewardTypeCourse           RewardType = "Course"           // Course / Coaching Program - Printed Materials
	RewardTypeCoaching         RewardType = "Coaching"         // Course / Coaching Program - Printed Materials
	RewardTypeBundle           RewardType = "Bundle"           // Course / Coaching Program - Printed Materials
	RewardTypePrintedMaterials RewardType = "PrintedMaterials" // Course / Coaching Program - Printed Materials
	RewardTypeDigital          RewardType = "Digital"          // Digital Product
	RewardTypeOther            RewardType = "Other"            // Other (أخرى)
)

// RewardStatus defines the status of a reward
type RewardStatus string

const (
	RewardStatusActive   RewardStatus = "Active"
	RewardStatusInactive RewardStatus = "Inactive"
	RewardStatusExpired  RewardStatus = "Expired"
)
