package models

type ComplaintStats struct {
	TicketsSummary        TicketsSummary        `json:"ticketsSummary"`
	TicketsResolutionTime TicketsResolutionTime `json:"ticketsResolutionTime"`
}

type TicketsSummary struct {
	TotalOpenTickets      int64 `json:"totalOpenTickets"`
	NumberOfClosedTickets int64 `json:"numberOfClosedTickets"`
	DeletedComplaints     int64 `json:"deletedComplaints"`
}

type TicketsResolutionTime struct {
	AverageTime string `json:"averageTime"`
	MaxTime     string `json:"maxTime"`
}
