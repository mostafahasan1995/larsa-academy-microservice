package enums

// CouponStatus defines possible statuses for a coupon

type CouponStatus string

const (
	CouponStatusActive  CouponStatus = "Active"
	CouponStatusExpired CouponStatus = "Expired"
)

// DiscountType defines type of discount: percent or fixed value

type DiscountType string

const (
	DiscountTypePercent DiscountType = "%"
	DiscountTypeFixed   DiscountType = "fixed"
)

// AutoApplyType defines whether coupon applies to all products or custom selection

type AutoApplyType string

const (
	AutoApplyAllProducts     AutoApplyType = "all"
	AutoApplySelectedProduct AutoApplyType = "selected"
)

// CouponType defines the type of coupon: general (for all products) or custom (for specific products)

type CouponType string

const (
	CouponTypeGeneral CouponType = "general"
	CouponTypeCustom  CouponType = "custom"
)
