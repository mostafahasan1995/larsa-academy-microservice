package content

// Calendar-related capabilities
const (
	// Admin capability - allows viewing all calendar events
	CapabilityViewAllCalendarEvents = "ViewAllCalendarEvents"

	// Teacher capability - allows viewing own calendar events
	CapabilityViewOwnCalendarEvents = "ViewOwnCalendarEvents"

	// Student capability - allows viewing own calendar events
	CapabilityViewStudentCalendarEvents = "ViewStudentCalendarEvents"

	// Supervisor capability - allows viewing calendar events for teachers they manage
	CapabilityViewSupervisedCalendarEvents = "ViewSupervisedCalendarEvents"

	// Parent capability - allows viewing calendar events for their children/students
	CapabilityViewChildrenCalendarEvents = "ViewChildrenCalendarEvents"

	// Placement test management capabilities
	// Admin-level: manage all placement test requests/schedules
	CapabilityManageAllPlacementTests = "ManageAllPlacementTests"
	// Supervisor-level: manage placement tests for teachers they oversee
	CapabilityManageSupervisedPlacementTests = "ManageSupervisedPlacementTests"

	// Teacher-facing API capability - allows accessing teacher-specific endpoints
	CapabilityAccessTeacherEndpoints = "AccessTeacherEndpoints"

	// Resource access capabilities
	CapabilityViewAllBundles          = "ViewAllBundles"
	CapabilityViewAllCourses          = "ViewAllCourses"
	CapabilityViewAllCoachingPrograms = "ViewAllCoachingPrograms"
	CapabilityViewAllStudents         = "ViewAllStudents"

	// Coaching management capabilities
	CapabilityManageAllCoachingPrograms = "ManageAllCoachingPrograms"
	CapabilityManageAssignedCoaching    = "ManageAssignedCoaching"
	CapabilityViewAssignedCoaching      = "ViewAssignedCoaching"
	CapabilityViewPublicCoaching        = "ViewPublicCoaching"
	CapabilityManageCoachingPricing     = "ManageCoachingPricing"
	CapabilityViewCoachingMembers       = "ViewCoachingMembers"

	// Bundle management capabilities
	CapabilityManageAllBundles    = "ManageAllBundles"
	CapabilityViewPublicBundles   = "ViewPublicBundles"
	CapabilityManageBundlePricing = "ManageBundlePricing"
	CapabilityViewBundleMembers   = "ViewBundleMembers"

	// Session management capabilities
	// Admin capability - allows managing all session details (notes, resources, agendas, feedback)
	CapabilityManageSessionDetails = "ManageSessionDetails"

	// Student question capabilities
	// Admin capability - allows replying to all student questions
	CapabilityReplyToAllStudentQuestions = "ReplyToAllStudentQuestions"

	// Course management capabilities
	// Admin capability - allows managing all courses (create, update, delete)
	CapabilityManageAllCourses = "ManageAllCourses"
	// Teacher/supervisor capability - allows managing courses assigned to them
	CapabilityManageAssignedCourses = "ManageAssignedCourses"
	// Resource editing capabilities - allows editing course resources (FAQs, flashcards, study notes, etc.) for assigned courses
	CapabilityEditCourseResources = "EditCourseResources"
	// Course pricing capabilities - admin-only
	CapabilityManageCoursePricing = "ManageCoursePricing"
	// Course member/enrollment viewing capabilities
	CapabilityViewCourseMembers = "ViewCourseMembers"
	// Viewing capabilities
	CapabilityViewAssignedCourses = "ViewAssignedCourses" // Teachers/supervisors see their courses
	CapabilityViewPublicCourses   = "ViewPublicCourses"   // Students see public courses

	// Student course data access capabilities
	CapabilityViewAllStudentCourseData = "ViewAllStudentCourseData" // Admin: view any student's course data

	// Coaching member remove capabilities
	// Admin: remove any member from any coaching
	CapabilityRemoveAnyMemberFromCoaching = "RemoveAnyMemberFromCoaching"
	// Teacher/supervisor: remove members from coachings assigned to them
	CapabilityRemoveManagedMemberFromCoaching = "RemoveManagedMemberFromCoaching"

	// Submission management capabilities
	// Admin: delete any submission
	CapabilityDeleteAnySubmission = "ElearningDeleteAnySubmission"
	// Teacher/supervisor: delete submissions for courses they are assigned to
	CapabilityDeleteAssignedCourseSubmissions = "ElearningDeleteAssignedCourseSubmissions"
)
