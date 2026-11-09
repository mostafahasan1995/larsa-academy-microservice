package models

import "time"

type StatusSummary struct {
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Late      int `json:"late"`
}

type UpcomingSession struct {
	StudentName string    `json:"studentName"`
	TeacherName string    `json:"teacherName"`
	Time        time.Time `json:"time"`
	Status      string    `json:"status"`
}

type RecentFeedback struct {
	StudentName string    `json:"studentName"`
	Rating      int       `json:"rating"`
	Comment     string    `json:"comment"`
	Date        time.Time `json:"date"`
}

// SupervisorDashboard represents the supervisor's dashboard statistics and today's sessions
type SupervisorDashboard struct {
	TotalTeachers    int               `json:"totalTeachers"`
	TotalStudents    int               `json:"totalStudents"`
	AssignedSubjects int               `json:"assignedSubjects"`
	ApprovedSubjects int               `json:"approvedSubjects"`
	TodaySessions    int               `json:"todaySessions"`
	StatusSummary    StatusSummary     `json:"statusSummary"`
	UpcomingSessions []UpcomingSession `json:"upcomingSessions"`
	RecentFeedbacks  []RecentFeedback  `json:"recentFeedbacks"`
	Sessions         []StudentSession  `json:"sessions"`
}
