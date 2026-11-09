package enums

// RuleStatus defines if a points rule is active
type RuleStatus string

const (
	RuleStatusActive   RuleStatus = "Active"
	RuleStatusInactive RuleStatus = "Inactive"
)

// PerformanceLevel defines performance tiers for automated point assignment
type PerformanceLevel string

const (
	PerformanceLevelGood      PerformanceLevel = "Good"      // جيد (< 60%)
	PerformanceLevelAverage   PerformanceLevel = "Average"   // متوسط (60% - 80%)
	PerformanceLevelExcellent PerformanceLevel = "Excellent" // ممتاز (> 80%)
	PerformanceLevelWeak      PerformanceLevel = "Weak"      // ضعيف (< 50%) - for commitment
)

// ActivityCategory defines main categories of point-earning activities
type ActivityCategory string

const (
	ActivityCategoryMainActivities       ActivityCategory = "MainActivities"       // الأنشطة الأساسية
	ActivityCategoryAdditionalActivities ActivityCategory = "AdditionalActivities" // أنشطة إضافية
)
