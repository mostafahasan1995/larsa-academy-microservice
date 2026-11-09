package enums

type MsgTyps int

const (
	None MsgTyps = iota
	INVITATION
	ACCOUNTUPDATED
	RENEWAL_EMAIL
	WELCOME_NEWSLETTER
	CONTACTUS
)
