package models

type Notification struct {
	ServiceName     string            `json:"serviceName"`
	Title           string            `json:"title"`
	Content         string            `json:"content"`
	Recipient       string            `json:"recipient"`
	Data            map[string]string `json:"data"`
	Url             string            `json:"url"`
	Badge           int               `json:"badge"`
	Sound           string            `json:"sound"`
	Image           string            `json:"image"`
	Icon            string            `json:"icon"`
	AndroidColor    string            `json:"androidColor"`
	AndroidSound    string            `json:"androidSound"`
	AndroidPriority string            `json:"androidPriority"`
	IOSSound        string            `json:"iosSound"`
}
