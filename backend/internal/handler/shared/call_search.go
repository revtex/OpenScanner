package shared

// CallSearchResult is a single call in the search response.
type CallSearchResult struct {
	ID             int64  `json:"id"`
	AudioName      string `json:"audioName"`
	AudioType      string `json:"audioType"`
	DateTime       int64  `json:"dateTime"`
	SystemID       int64  `json:"systemId"`
	SystemLabel    string `json:"systemLabel"`
	TalkgroupID    int64  `json:"talkgroupId"`
	TalkgroupLabel string `json:"talkgroupLabel"`
	TalkgroupName  string `json:"talkgroupName"`
	TalkgroupGroup string `json:"talkgroupGroup,omitempty"`
	TalkgroupTag   string `json:"talkgroupTag,omitempty"`
	TalkgroupLed   string `json:"talkgroupLed,omitempty"`
	Frequency      *int64 `json:"frequency,omitempty"`
	Duration       *int64 `json:"duration,omitempty"`
	Source         *int64 `json:"source,omitempty"`
	Site           string `json:"site,omitempty"`
	Channel        string `json:"channel,omitempty"`
	Decoder        string `json:"decoder,omitempty"`
	ErrorCount     *int64 `json:"errorCount,omitempty"`
	SpikeCount     *int64 `json:"spikeCount,omitempty"`
	TalkerAlias    string `json:"talkerAlias,omitempty"`
	Transcript     string `json:"transcript,omitempty"`
	Bookmarked     bool   `json:"bookmarked"`
} // @name CallSearchResult

// CallSearchResponse is the response for GET /api/calls.
type CallSearchResponse struct {
	Calls []CallSearchResult `json:"calls"`
	Total int64              `json:"total"`
} // @name CallSearchResponse
