package api

import (
	"database/sql"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// ErrorResponse is the standard JSON error envelope used by all endpoints.
type ErrorResponse struct {
	Error string `json:"error" example:"descriptive error message"`
} // @name ErrorResponse

// nullStr unwraps sql.NullString to *string for clean JSON serialisation.
func nullStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

// nullInt unwraps sql.NullInt64 to *int64 for clean JSON serialisation.
func nullInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

// ── User ──

type userResponse struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Role        string  `json:"role"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson"`
	Expiration  *int64  `json:"expiration"`
	Limit       *int64  `json:"limit"`
	CreatedAt   int64   `json:"createdAt"`
	UpdatedAt   int64   `json:"updatedAt"`
} // @name UserResponse

func toUserResponse(u db.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Username:    u.Username,
		Role:        u.Role,
		Disabled:    u.Disabled,
		SystemsJson: nullStr(u.SystemsJson),
		Expiration:  nullInt(u.Expiration),
		Limit:       nullInt(u.Limit),
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func toUserResponses(users []db.User) []userResponse {
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = toUserResponse(u)
	}
	return out
}

// ── System ──

type systemResponse struct {
	ID             int64   `json:"id"`
	SystemID       int64   `json:"systemId"`
	Label          string  `json:"label"`
	AutoPopulate   int64   `json:"autoPopulate"`
	BlacklistsJson *string `json:"blacklistsJson"`
	Led            *string `json:"led"`
	Order          int64   `json:"order"`
} // @name SystemResponse

func toSystemResponse(s db.System) systemResponse {
	return systemResponse{
		ID:             s.ID,
		SystemID:       s.SystemID,
		Label:          s.Label,
		AutoPopulate:   s.AutoPopulate,
		BlacklistsJson: nullStr(s.BlacklistsJson),
		Led:            nullStr(s.Led),
		Order:          s.Order,
	}
}

func toSystemResponses(systems []db.System) []systemResponse {
	out := make([]systemResponse, len(systems))
	for i, s := range systems {
		out[i] = toSystemResponse(s)
	}
	return out
}

// ── Talkgroup ──

type talkgroupResponse struct {
	ID          int64   `json:"id"`
	SystemID    int64   `json:"systemId"`
	TalkgroupID int64   `json:"talkgroupId"`
	Label       *string `json:"label"`
	Name        *string `json:"name"`
	Frequency   *int64  `json:"frequency"`
	Led         *string `json:"led"`
	GroupID     *int64  `json:"groupId"`
	TagID       *int64  `json:"tagId"`
	Order       int64   `json:"order"`
} // @name TalkgroupResponse

func toTalkgroupResponse(t db.Talkgroup) talkgroupResponse {
	return talkgroupResponse{
		ID:          t.ID,
		SystemID:    t.SystemID,
		TalkgroupID: t.TalkgroupID,
		Label:       nullStr(t.Label),
		Name:        nullStr(t.Name),
		Frequency:   nullInt(t.Frequency),
		Led:         nullStr(t.Led),
		GroupID:     nullInt(t.GroupID),
		TagID:       nullInt(t.TagID),
		Order:       t.Order,
	}
}

func toTalkgroupResponses(talkgroups []db.Talkgroup) []talkgroupResponse {
	out := make([]talkgroupResponse, len(talkgroups))
	for i, t := range talkgroups {
		out[i] = toTalkgroupResponse(t)
	}
	return out
}

// ── Unit ──

type unitResponse struct {
	ID       int64   `json:"id"`
	SystemID int64   `json:"systemId"`
	UnitID   int64   `json:"unitId"`
	Label    *string `json:"label"`
	Order    int64   `json:"order"`
} // @name UnitResponse

func toUnitResponse(u db.Unit) unitResponse {
	return unitResponse{
		ID:       u.ID,
		SystemID: u.SystemID,
		UnitID:   u.UnitID,
		Label:    nullStr(u.Label),
		Order:    u.Order,
	}
}

func toUnitResponses(units []db.Unit) []unitResponse {
	out := make([]unitResponse, len(units))
	for i, u := range units {
		out[i] = toUnitResponse(u)
	}
	return out
}

// ── API Key ──

type apiKeyResponse struct {
	ID            int64   `json:"id"`
	Fingerprint   string  `json:"fingerprint"`
	Ident         *string `json:"ident"`
	Disabled      int64   `json:"disabled"`
	SystemsJson   *string `json:"systemsJson"`
	CallRateLimit *int64  `json:"callRateLimit"`
	Order         int64   `json:"order"`
} // @name ApiKeyResponse

type apiKeyCreateResponse struct {
	apiKeyResponse
	CreatedKey string `json:"createdKey"`
} // @name ApiKeyCreateResponse

func toAPIKeyResponse(k db.ApiKey) apiKeyResponse {
	fingerprint := auth.HashAPIKey(k.Key)
	if len(fingerprint) > 12 {
		fingerprint = fingerprint[:12]
	}
	return apiKeyResponse{
		ID:            k.ID,
		Fingerprint:   fingerprint,
		Ident:         nullStr(k.Ident),
		Disabled:      k.Disabled,
		SystemsJson:   nullStr(k.SystemsJson),
		CallRateLimit: nullInt(k.CallRateLimit),
		Order:         k.Order,
	}
}

func toAPIKeyResponses(keys []db.ApiKey) []apiKeyResponse {
	out := make([]apiKeyResponse, len(keys))
	for i, k := range keys {
		out[i] = toAPIKeyResponse(k)
	}
	return out
}

// ── DirMonitor ──

type dirmonitorResponse struct {
	ID          int64   `json:"id"`
	Directory   string  `json:"directory"`
	Type        string  `json:"type"`
	Mask        *string `json:"mask"`
	Extension   *string `json:"extension"`
	Frequency   *int64  `json:"frequency"`
	Delay       *int64  `json:"delay"`
	DeleteAfter int64   `json:"deleteAfter"`
	UsePolling  int64   `json:"usePolling"`
	Disabled    int64   `json:"disabled"`
	SystemID    *int64  `json:"systemId"`
	TalkgroupID *int64  `json:"talkgroupId"`
	Order       int64   `json:"order"`
} // @name DirMonitorResponse

func toDirMonitorResponse(d db.Dirmonitor) dirmonitorResponse {
	return dirmonitorResponse{
		ID:          d.ID,
		Directory:   d.Directory,
		Type:        d.Type,
		Mask:        nullStr(d.Mask),
		Extension:   nullStr(d.Extension),
		Frequency:   nullInt(d.Frequency),
		Delay:       nullInt(d.Delay),
		DeleteAfter: d.DeleteAfter,
		UsePolling:  d.UsePolling,
		Disabled:    d.Disabled,
		SystemID:    nullInt(d.SystemID),
		TalkgroupID: nullInt(d.TalkgroupID),
		Order:       d.Order,
	}
}

func toDirMonitorResponses(dirmonitors []db.Dirmonitor) []dirmonitorResponse {
	out := make([]dirmonitorResponse, len(dirmonitors))
	for i, d := range dirmonitors {
		out[i] = toDirMonitorResponse(d)
	}
	return out
}

// ── Downstream ──

type downstreamResponse struct {
	ID          int64   `json:"id"`
	Url         string  `json:"url"`
	ApiKey      string  `json:"apiKey"`
	SystemsJson *string `json:"systemsJson"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
} // @name DownstreamResponse

func toDownstreamResponse(d db.Downstream) downstreamResponse {
	return downstreamResponse{
		ID:          d.ID,
		Url:         d.Url,
		ApiKey:      d.ApiKey,
		SystemsJson: nullStr(d.SystemsJson),
		Disabled:    d.Disabled,
		Order:       d.Order,
	}
}

func toDownstreamResponses(downstreams []db.Downstream) []downstreamResponse {
	out := make([]downstreamResponse, len(downstreams))
	for i, d := range downstreams {
		out[i] = toDownstreamResponse(d)
	}
	return out
}

// ── Webhook ──

type webhookResponse struct {
	ID          int64   `json:"id"`
	Url         string  `json:"url"`
	Type        string  `json:"type"`
	Secret      *string `json:"secret"`
	SystemsJson *string `json:"systemsJson"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
} // @name WebhookResponse

func toWebhookResponse(w db.Webhook) webhookResponse {
	return webhookResponse{
		ID:          w.ID,
		Url:         w.Url,
		Type:        w.Type,
		Secret:      nullStr(w.Secret),
		SystemsJson: nullStr(w.SystemsJson),
		Disabled:    w.Disabled,
		Order:       w.Order,
	}
}

func toWebhookResponses(webhooks []db.Webhook) []webhookResponse {
	out := make([]webhookResponse, len(webhooks))
	for i, w := range webhooks {
		out[i] = toWebhookResponse(w)
	}
	return out
}

// ── Log ──

type logResponse struct {
	ID       int64  `json:"id"`
	DateTime int64  `json:"dateTime"`
	Level    string `json:"level"`
	Message  string `json:"message"`
} // @name LogResponse

func toLogResponse(l db.Log) logResponse {
	return logResponse{
		ID:       l.ID,
		DateTime: l.DateTime,
		Level:    l.Level,
		Message:  l.Message,
	}
}

func toLogResponses(logs []db.Log) []logResponse {
	out := make([]logResponse, len(logs))
	for i, l := range logs {
		out[i] = toLogResponse(l)
	}
	return out
}

// ── Settings / Config ──

type settingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
} // @name SettingResponse

func toSettingResponses(settings []db.Setting) []settingResponse {
	out := make([]settingResponse, len(settings))
	for i, s := range settings {
		out[i] = settingResponse{Key: s.Key, Value: s.Value}
	}
	return out
}
