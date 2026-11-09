package models

import (
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Supervisor struct {
	Id            primitive.ObjectID `bson:"_id" json:"_id"`
	Seq           string             `bson:"seq" json:"seq"`
	SupervisorDTO `bson:",inline" json:",inline"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy     primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy     primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash         bool               `bson:"trash" json:"trash"`
}

func (s *Supervisor) CollName() string {
	return "academySupervisors"
}

// SupervisorWithStats extends Supervisor with additional calculated statistics
type SupervisorWithStats struct {
	Supervisor       `bson:",inline"`
	SubjectAssigned  []string `bson:"subjectAssigned,omitempty" json:"subjectAssigned,omitempty"`
	AssignedStage    []string `bson:"assignedStage,omitempty" json:"assignedStage,omitempty"`
	TotalNumStudents int      `bson:"totalNumStudents" json:"totalNumStudents"`
	TotalNumTeachers int      `bson:"totalNumTeachers" json:"totalNumTeachers"`
}

type SupervisorWithPagination struct {
	Supervisors []SupervisorWithStats `bson:"supervisors" json:"supervisors"`
	Pagination  common.Pagination     `bson:"pagination" json:"pagination"`
}

type SupervisorDTO struct {
	FirstName          transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName           transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Gender             string                     `bson:"gender" json:"gender"`
	DateOfBirth        time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality        string                     `bson:"nationality" json:"nationality"`
	NationalID         string                     `bson:"nationalID" json:"nationalID"`
	Status             enums.SupervisorStatus     `bson:"status" json:"status" validate:"required,oneof=active inactive"`
	EmploymentStatus   string                     `bson:"employmentStatus" json:"employmentStatus"`
	ProfilePicture     types.FileField            `bson:"profilePicture" json:"profilePicture"`
	IdDocument         types.FileField            `bson:"IdDocument" json:"IdDocument"`
	MobileNumber       PhoneNumber                `bson:"mobileNumber" json:"mobileNumber"`
	Subjects           []Subject                  `bson:"subjects" json:"subjects"`
	Address            Address                    `bson:"address" json:"address"`
	AccountSettings    AccountSettings            `bson:"accountSettings" json:"accountSettings"`
	RoleAndPermissions RoleAndPermissions         `bson:"roleAndPermissions" json:"roleAndPermissions"`
	SupQualification   SupQualificationDTO        `bson:"supQualification" json:"supQualification"`
}

type SupQualificationDTO struct {
	HighestDegree          string            `bson:"highestDegree" json:"highestDegree"`
	YearOfExperience       int               `bson:"yearOfExperience" json:"yearOfExperience"`
	University             string            `bson:"university" json:"university"`
	Cv                     types.FileField   `bson:"cv" json:"cv"`
	Certificates           []types.FileField `bson:"certificates" json:"certificates"`
	Contracts              []types.FileField `bson:"contracts" json:"contracts"`
	Skills                 string            `bson:"skills" json:"skills"`
	AcademicSpecialization string            `bson:"academicSpecialization" json:"academicSpecialization"`
	Description            string            `bson:"description" json:"description"`
}

func (s *Supervisor) GetUserData() map[string]any {
	// Extract default language (ar) from Localizable fields
	firstName := s.FirstName.GetContentByLang("ar")
	lastName := s.LastName.GetContentByLang("ar")

	m := map[string]any{
		"firstName": firstName,
		"lastName":  lastName,
		"email":     s.AccountSettings.Email,
	}

	// Add picture if available
	if s.ProfilePicture.Path != "" {
		m["picture"] = s.ProfilePicture.Path
	}

	// add phone if available
	if s.MobileNumber.Content != "" && s.MobileNumber.Prefix != "" {
		m["phone"] = s.MobileNumber.Prefix + s.MobileNumber.Content
	}

	if s.AccountSettings.InitialPass != "" {
		m["password"] = s.AccountSettings.InitialPass
		m["passwordConfirmation"] = s.AccountSettings.ConfirmPass
	}

	return m
}

type SupervisorSummary struct {
	Supervisor       Supervisor    `bson:"supervisor" json:"supervisor"`
	AssignedStudents []interface{} `bson:"assignedStudents" json:"assignedStudents"`
	AssignedTeachers []interface{} `bson:"assignedTeachers" json:"assignedTeachers"`
	AssignedCourses  []interface{} `bson:"assignedCourses" json:"assignedCourses"`
}

type ClassGradeInfo struct {
	Grade            string                     `json:"grade"`
	ContentName      transl.Localizable[string] `json:"contentName"`
	NumberOfStudents int                        `json:"numberOfStudents"`
	TeacherAssigned  int                        `json:"teacherAssigned"`
}

type ClassGradeInfoPagination struct {
	ClassGradeInfos []ClassGradeInfo  `json:"classGradeInfos"`
	Pagination      common.Pagination `json:"pagination"`
}

// ManagedUsersResponse represents the IDs of users managed by a supervisor
type ManagedUsersResponse struct {
	TeacherIds []primitive.ObjectID `json:"teacherIds"`
	StudentIds []primitive.ObjectID `json:"studentIds,omitempty"` // Only included if withStudent=true
}

// SupervisorTeachersExtended represents teachers separated into direct and indirect
type SupervisorTeachersExtended struct {
	DirectTeachers   []Teacher `json:"directTeachers"`
	IndirectTeachers []Teacher `json:"indirectTeachers"`
	DirectCount      int       `json:"directCount"`
	IndirectCount    int       `json:"indirectCount"`
}

// SupervisorStudentsExtended represents students from courses and coachings
type SupervisorStudentsExtended struct {
	Students []Student `json:"students"`
	Count    int       `json:"count"`
}
