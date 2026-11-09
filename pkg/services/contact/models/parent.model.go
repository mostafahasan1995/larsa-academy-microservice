package models

import (
	// "larsa-academy-microservice/pkg/services/contact/enums"
	// "larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ParentDto struct {
	ParentGeneralInfo    GeneralInfo          `bson:"parentGeneralInfo" json:"parentGeneralInfo"`
	AccountParentSetting AccountParentSetting `bson:"accountParentSetting" json:"accountParentSetting"`
	ParentContactInfo    ParentContactInfo    `bson:"parentContactInfo" json:"parentContactInfo"`
	ParentAddressDetails ParentAddressDetails `bson:"parentAddressDetails" json:"parentAddressDetails"`
}

type ParentGeneralInfo struct {
	FirstName   transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName    transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Gender      string                     `bson:"gender" json:"gender"`
	DateOfBirth time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality string                     `bson:"nationality" json:"nationality"`
	Status      enums.ParentStatus         `bson:"status" json:"status" validate:"required,oneof=active pending suspend"`
	Picture     types.FileField            `bson:"picture" json:"picture"`
}

type ParentContactInfo struct {
	MobilePhoneNumber PhoneNumber `bson:"mobilePhoneNumber" json:"mobilePhoneNumber"`
	EmailAddress      string      `bson:"emailAddress" json:"emailAddress"`
}

type ParentAddressDetails struct {
	Country        string `bson:"country" json:"country"`
	City           string `bson:"city" json:"city"`
	Street         string `bson:"street" json:"street"`
	AdditionalInfo string `bson:"additionalInfo" json:"additionalInfo"`
}

type AccountParentSetting struct {
	IsActive          bool   `bson:"isActive" json:"isActive"`
	Email             string `bson:"email" json:"email"`
	InitialPass       string `bson:"initialPass" json:"initialPass"`
	ConfirmPass       string `bson:"confirmPass" json:"confirmPass"`
	ResetOnFirstLogin bool   `bson:"resetOnFirstLogin" json:"resetOnFirstLogin"`
	JoinStatus        string `bson:"joinStatus" json:"joinStatus"`
}

type ParentRelation struct {
	StudentId    primitive.ObjectID `bson:"studentId" json:"studentId"`
	RelationType string             `bson:"relationType" json:"relationType"`
}

type Parent struct {
	Id                     primitive.ObjectID       `bson:"_id" json:"_id"`
	Seq                    string                   `bson:"seq" json:"seq"`
	ParentStudentRelations []*ParentStudentRelation `bson:"parentStudentRelations" json:"parentStudentRelations"`
	ParentDto              `bson:",inline"`
	CreatedAt              time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt              time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy              primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy              primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash                  bool               `bson:"trash" json:"trash"`
}

func (p *Parent) CollName() string {
	return "academyParents"
}

// ParentWithStats extends Parent with additional calculated statistics
type ParentWithStats struct {
	Parent           `bson:",inline"`
	NumberOfChildren int `bson:"numberOfChildren,omitempty" json:"numberOfChildren,omitempty"`
}

type ParentsPagination struct {
	Parents    []ParentWithStats `bson:"parents" json:"parents"`
	Pagination types.Pagination  `bson:"pagination" json:"pagination"`
}

type ParentSummary struct {
	Parent     Parent                  `bson:"parent" json:"parent"`
	Complaints []interface{}           `bson:"complaints" json:"complaints"`
	Children   []ParentStudentRelation `bson:"children" json:"children"`
	// add messages received
}

type ChildWithStats struct {
	ParentStudentRelation `bson:",inline"`
	EnrolledCoursesCount  int       `json:"enrolledCoursesCount"`
	Grade                 string    `json:"grade"`
	AttendancePercentage  float64   `json:"attendancePercentage"`
	AcademicProgress      float64   `json:"academicProgress"` // percentage
	TotalPoints           int       `json:"totalPoints"`
	LastActive            time.Time `json:"lastActive"`
	Status                string    `json:"status"`
}

type ChildrenPagination struct {
	Children   []ChildWithStats `json:"children"`
	Pagination types.Pagination `json:"pagination"`
}

type StudentInfo struct {
	Id        string                     `json:"id"`
	FirstName transl.Localizable[string] `json:"firstName"`
	LastName  transl.Localizable[string] `json:"lastName"`
}

type ParentInfo struct {
	Id        string                     `json:"id"`
	FirstName transl.Localizable[string] `json:"firstName"`
	LastName  transl.Localizable[string] `json:"lastName"`
}

type ParentShortInfo struct {
	Id        primitive.ObjectID         `json:"id"`
	FirstName transl.Localizable[string] `json:"firstName"`
	LastName  transl.Localizable[string] `json:"lastName"`
	Phone     PhoneNumber                `json:"phone"`
}

type ChildResponse struct {
	Id                       string      `json:"_id"`
	Student                  StudentInfo `json:"student"`
	Parent                   ParentInfo  `json:"parent"`
	RelationType             string      `json:"relationType"`
	EnrolledCoursesCount     int         `json:"enrolledCoursesCount"`
	EnrolledCoachingsCount   int         `json:"enrolledCoachingsCount"`
	Grade                    string      `json:"grade"`
	AttendancePercentage     float64     `json:"attendancePercentage"`
	AcademicProgress         float64     `json:"academicProgress"`
	CoachingCommitmentProgress float64   `json:"coachingCommitmentProgress"`
	TotalPoints              int         `json:"totalPoints"`
	LastActive               time.Time   `json:"lastActive"`
	Status                   string      `json:"status"`
	CreatedAt                time.Time   `json:"createdAt"`
	UpdatedAt                time.Time   `json:"updatedAt"`
}

type ChildrenResponse struct {
	Children   []ChildResponse  `json:"children"`
	Pagination types.Pagination `json:"pagination"`
}

func (s *Parent) GetUserData() map[string]any {
	m := map[string]any{
		"firstName": s.ParentGeneralInfo.FirstName.GetContentByLang("ar"),
		"lastName":  s.ParentGeneralInfo.LastName.GetContentByLang("ar"),
		"email":     s.AccountParentSetting.Email,
	}

	if s.ParentContactInfo.MobilePhoneNumber.Content != "" && s.ParentContactInfo.MobilePhoneNumber.Prefix != "" {
		m["phone"] = s.ParentContactInfo.MobilePhoneNumber.Prefix + s.ParentContactInfo.MobilePhoneNumber.Content
	}
	if s.AccountParentSetting.InitialPass != "" {
		m["password"] = s.AccountParentSetting.InitialPass
		m["passwordConfirmation"] = s.AccountParentSetting.ConfirmPass
	}

	return m
}
