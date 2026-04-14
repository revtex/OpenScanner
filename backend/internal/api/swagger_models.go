package api

// Swagger model types — clean mirrors of db types for Swagger documentation.
// These are referenced via .swaggo replace directives and never used at runtime.

// swagGroup mirrors db.Group for Swagger.
type swagGroup struct { //nolint:unused
	ID    int64  `json:"id"`
	Label string `json:"label"`
} // @name Group

// swagTag mirrors db.Tag for Swagger.
type swagTag struct { //nolint:unused
	ID    int64  `json:"id"`
	Label string `json:"label"`
} // @name Tag

// swagSetting mirrors db.Setting for Swagger.
type swagSetting struct { //nolint:unused
	Key   string `json:"key"`
	Value string `json:"value"`
} // @name Setting

// swagUser mirrors db.User for Swagger.
type swagUser struct { //nolint:unused
	ID                 int64   `json:"id"`
	Username           string  `json:"username"`
	Role               string  `json:"role"`
	Disabled           int64   `json:"disabled"`
	SystemsJson        *string `json:"systems_json"`
	Expiration         *int64  `json:"expiration"`
	Limit              *int64  `json:"limit"`
	PasswordNeedChange int64   `json:"password_need_change"`
	CreatedAt          int64   `json:"created_at"`
	UpdatedAt          int64   `json:"updated_at"`
} // @name User

// swagSystem mirrors db.System for Swagger.
type swagSystem struct { //nolint:unused
	ID             int64   `json:"id"`
	SystemID       int64   `json:"system_id"`
	Label          string  `json:"label"`
	AutoPopulate   int64   `json:"auto_populate"`
	BlacklistsJson *string `json:"blacklists_json"`
	Led            *string `json:"led"`
	Order          int64   `json:"order"`
} // @name System

// swagTalkgroup mirrors db.Talkgroup for Swagger.
type swagTalkgroup struct { //nolint:unused
	ID          int64   `json:"id"`
	SystemID    int64   `json:"system_id"`
	TalkgroupID int64   `json:"talkgroup_id"`
	Label       *string `json:"label"`
	Name        *string `json:"name"`
	Frequency   *int64  `json:"frequency"`
	Led         *string `json:"led"`
	GroupID     *int64  `json:"group_id"`
	TagID       *int64  `json:"tag_id"`
	Order       int64   `json:"order"`
} // @name Talkgroup

// swagUnit mirrors db.Unit for Swagger.
type swagUnit struct { //nolint:unused
	ID       int64   `json:"id"`
	SystemID int64   `json:"system_id"`
	UnitID   int64   `json:"unit_id"`
	Label    *string `json:"label"`
	Order    int64   `json:"order"`
} // @name Unit

// swagApiKey mirrors db.ApiKey for Swagger.
type swagApiKey struct { //nolint:unused
	ID          int64   `json:"id"`
	Key         string  `json:"key"`
	Ident       *string `json:"ident"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systems_json"`
	Order       int64   `json:"order"`
} // @name ApiKey

// swagDirmonitor mirrors db.Dirmonitor for Swagger.
type swagDirmonitor struct { //nolint:unused
	ID          int64   `json:"id"`
	Directory   string  `json:"directory"`
	Type        string  `json:"type"`
	Mask        *string `json:"mask"`
	Extension   *string `json:"extension"`
	Frequency   *int64  `json:"frequency"`
	Delay       *int64  `json:"delay"`
	DeleteAfter int64   `json:"delete_after"`
	UsePolling  int64   `json:"use_polling"`
	Disabled    int64   `json:"disabled"`
	SystemID    *int64  `json:"system_id"`
	TalkgroupID *int64  `json:"talkgroup_id"`
	Order       int64   `json:"order"`
} // @name DirMonitor

// swagDownstream mirrors db.Downstream for Swagger.
type swagDownstream struct { //nolint:unused
	ID          int64   `json:"id"`
	Url         string  `json:"url"`
	ApiKey      string  `json:"api_key"`
	SystemsJson *string `json:"systems_json"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
} // @name Downstream

// swagWebhook mirrors db.Webhook for Swagger.
type swagWebhook struct { //nolint:unused
	ID          int64   `json:"id"`
	Url         string  `json:"url"`
	Type        string  `json:"type"`
	Secret      *string `json:"secret"`
	SystemsJson *string `json:"systems_json"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
} // @name Webhook

var (
	_ swagGroup
	_ swagTag
	_ swagSetting
	_ swagUser
	_ swagSystem
	_ swagTalkgroup
	_ swagUnit
	_ swagApiKey
	_ swagDirmonitor
	_ swagDownstream
	_ swagWebhook
)
