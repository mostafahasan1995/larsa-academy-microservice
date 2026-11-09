package content

import (
	"context"
	"errors"

	"larsa-academy-microservice/pkg/db"
	contactsvcs "larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CourseAuthService defines the interface for course authorization operations
type CourseAuthService interface {
	CanViewCourse(ctx context.Context, courseId string) (bool, error)
	CanManageCourse(ctx context.Context, courseId string) (bool, error)
	CanEditCourseResources(ctx context.Context, courseId string) (bool, error)
	CanManageCoursePricing(ctx context.Context, courseId string) (bool, error)
	CanViewCourseMembers(ctx context.Context, courseId string) (bool, error)
	CanViewStudentCourseData(ctx context.Context, studentId string) (bool, error)
	FilterCoursesByPermissions(ctx context.Context, courses []models.Course) ([]models.Course, error)
}

type courseAuthService struct {
	i                    *do.Injector
	teacherAssignmentSvc TeacherAssignmentSvcs
	courseRepo           db.MainRepo[models.Course]
	studentCourseRepo    db.MainRepo[models.StudentCourse]
	parentSvcs           contactsvcs.ParentSvcs
}

func NewCourseAuthService(i *do.Injector) (CourseAuthService, error) {
	return &courseAuthService{
		i:                    i,
		teacherAssignmentSvc: do.MustInvoke[TeacherAssignmentSvcs](i),
		courseRepo:           db.NewRepo[models.Course](i),
		studentCourseRepo:    db.NewRepo[models.StudentCourse](i),
		parentSvcs:           do.MustInvoke[contactsvcs.ParentSvcs](i),
	}, nil
}

// CanViewCourse checks if the current user can view a specific course (metadata)
func (s *courseAuthService) CanViewCourse(ctx context.Context, courseId string) (bool, error) {
	// Admin can view all courses
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCourses); canViewAll {
		return true, nil
	}

	// Check if teacher/supervisor can view assigned courses
	if canViewAssigned, _ := util.HasCapability(ctx, CapabilityViewAssignedCourses); canViewAssigned {
		return s.isAssignedToCourse(ctx, courseId)
	}

	// Check if student can view public courses or courses they are enrolled in (even if not active)
	if canViewPublic, _ := util.HasCapability(ctx, CapabilityViewPublicCourses); canViewPublic {
		// Students can view public courses
		if isPublic, _ := s.isCoursePublic(ctx, courseId); isPublic {
			return true, nil
		}
		// Students can also view courses they are/were enrolled in
		if isEnrolled, _ := s.isStudentEnrolledInCourse(ctx, courseId); isEnrolled {
			return true, nil
		}
	}

	return false, nil
}

// CanManageCourse checks if the current user can manage (create/update/delete) a specific course
func (s *courseAuthService) CanManageCourse(ctx context.Context, courseId string) (bool, error) {
	// Admin can manage all courses
	if canManageAll, _ := util.HasCapability(ctx, CapabilityManageAllCourses); canManageAll {
		return true, nil
	}

	// Teachers/supervisors can manage courses assigned to them
	if canManageAssigned, _ := util.HasCapability(ctx, CapabilityManageAssignedCourses); canManageAssigned {
		return s.isAssignedToCourse(ctx, courseId)
	}

	return false, nil
}

// CanEditCourseResources checks if the current user can edit resources (FAQs, flashcards, etc.) for a specific course
func (s *courseAuthService) CanEditCourseResources(ctx context.Context, courseId string) (bool, error) {
	// Admin can edit all course resources
	if canManageAll, _ := util.HasCapability(ctx, CapabilityManageAllCourses); canManageAll {
		return true, nil
	}

	// Teachers/supervisors can edit resources for courses they're assigned to
	if canEditResources, _ := util.HasCapability(ctx, CapabilityEditCourseResources); canEditResources {
		return s.isAssignedToCourse(ctx, courseId)
	}

	return false, nil
}

// CanManageCoursePricing checks if the current user can manage pricing for a specific course (admin-only)
func (s *courseAuthService) CanManageCoursePricing(ctx context.Context, courseId string) (bool, error) {
	// Only admin can manage course pricing
	canManagePricing, err := util.HasCapability(ctx, CapabilityManageCoursePricing)
	if err != nil {
		return false, err
	}
	return canManagePricing, nil
}

// CanViewCourseMembers checks if the current user can view course members/enrollments
func (s *courseAuthService) CanViewCourseMembers(ctx context.Context, courseId string) (bool, error) {
	// Admin can view all course members
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCourses); canViewAll {
		return true, nil
	}

	// Teachers/supervisors can view members of courses they're assigned to
	if canViewMembers, _ := util.HasCapability(ctx, CapabilityViewCourseMembers); canViewMembers {
		return s.isAssignedToCourse(ctx, courseId)
	}

	return false, nil
}

// CanViewStudentCourseData checks if the current user can view a student's course data
func (s *courseAuthService) CanViewStudentCourseData(ctx context.Context, studentId string) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return false, errors.New("invalid student id")
	}

	// Student can view their own data
	if cfg.User.Id.Hex() == studentId {
		return true, nil
	}

	// Admin can view all student course data
	canViewAll, err := util.HasCapability(ctx, CapabilityViewAllStudentCourseData)
	if err != nil {
		return false, err
	}
	if canViewAll {
		return true, nil
	}

	// Parents can view their children's course data
	isParent, err := s.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, _studentId)
	if err != nil {
		return false, err
	}
	if isParent {
		return true, nil
	}

	// Teachers can view data of students in their assigned courses
	// This requires checking if the student is enrolled in courses assigned to the teacher
	// For now, return false - this will be checked at the service level
	return false, nil
}

// FilterCoursesByPermissions filters a list of courses based on the current user's permissions
func (s *courseAuthService) FilterCoursesByPermissions(ctx context.Context, courses []models.Course) ([]models.Course, error) {
	var filteredCourses []models.Course

	for _, course := range courses {
		canView, err := s.CanViewCourse(ctx, course.Id.Hex())
		if err != nil {
			return nil, err
		}
		if canView {
			filteredCourses = append(filteredCourses, course)
		}
	}

	return filteredCourses, nil
}

// isAssignedToCourse checks if the current user is assigned to a course as a teacher or supervisor
func (s *courseAuthService) isAssignedToCourse(ctx context.Context, courseId string) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return false, errors.New("invalid course id")
	}

	// Check if user is a teacher assigned to this course
	teacherIds, err := s.teacherAssignmentSvc.GetCourseTeachers(ctx, _id)
	if err != nil {
		return false, err
	}

	for _, teacherId := range teacherIds {
		if teacherId == cfg.User.Id {
			return true, nil
		}
	}

	// Check if user is a supervisor for this course
	course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return false, err
	}

	for _, supervisorId := range course.BasicInfo.SupervisorIds {
		if supervisorId == cfg.User.Id {
			return true, nil
		}
	}

	return false, nil
}

// isCoursePublic checks if a course is marked as public (active status = public)
func (s *courseAuthService) isCoursePublic(ctx context.Context, courseId string) (bool, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return false, errors.New("invalid course id")
	}

	course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return false, err
	}

	// Check if course status is active (active = public)
	return course.BasicInfo.Status == "active", nil
}

// isStudentEnrolledInCourse checks if the current user has any enrollment record for a course
func (s *courseAuthService) isStudentEnrolledInCourse(ctx context.Context, courseId string) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return false, errors.New("invalid course id")
	}

	// Check if student has any enrollment record for this course (non-trashed)
	count, err := s.studentCourseRepo.Count(ctx, bson.M{
		"studentId": cfg.User.Id,
		"courseId":  _id,
		"trash":     false,
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// isStudentActiveInCourse checks if the current user has an active enrollment in a course
func (s *courseAuthService) isStudentActiveInCourse(ctx context.Context, courseId string) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return false, errors.New("invalid course id")
	}

	// Check if student has an active enrollment record for this course
	count, err := s.studentCourseRepo.Count(ctx, bson.M{
		"studentId": cfg.User.Id,
		"courseId":  _id,
		"status":    enums.EnrollmentStatusActive,
		"trash":     false,
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
