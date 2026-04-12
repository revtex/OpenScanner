package api

import (
	"database/sql"

	"github.com/openscanner/openscanner/internal/db"
)

// ptrToNullStr converts *string to sql.NullString for DB params.
func ptrToNullStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

// ptrToNullInt converts *int64 to sql.NullInt64 for DB params.
func ptrToNullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// ── System requests ──

type createSystemRequest struct {
	SystemID       int64   `json:"systemId"`
	Label          string  `json:"label"`
	AutoPopulate   int64   `json:"autoPopulate"`
	BlacklistsJson *string `json:"blacklistsJson"`
	Led            *string `json:"led"`
	Order          int64   `json:"order"`
}

func (r createSystemRequest) toParams() db.CreateSystemParams {
	return db.CreateSystemParams{
		SystemID:       r.SystemID,
		Label:          r.Label,
		AutoPopulate:   r.AutoPopulate,
		BlacklistsJson: ptrToNullStr(r.BlacklistsJson),
		Led:            ptrToNullStr(r.Led),
		Order:          r.Order,
	}
}

type updateSystemRequest struct {
	SystemID       int64   `json:"systemId"`
	Label          string  `json:"label"`
	AutoPopulate   int64   `json:"autoPopulate"`
	BlacklistsJson *string `json:"blacklistsJson"`
	Led            *string `json:"led"`
	Order          int64   `json:"order"`
}

type reorderSystemItem struct {
	ID    int64 `json:"id"`
	Order int64 `json:"order"`
}

type reorderSystemsRequest struct {
	Systems []reorderSystemItem `json:"systems"`
}

func (r updateSystemRequest) toParams(id int64) db.UpdateSystemParams {
	return db.UpdateSystemParams{
		ID:             id,
		SystemID:       r.SystemID,
		Label:          r.Label,
		AutoPopulate:   r.AutoPopulate,
		BlacklistsJson: ptrToNullStr(r.BlacklistsJson),
		Led:            ptrToNullStr(r.Led),
		Order:          r.Order,
	}
}

// ── Talkgroup requests ──

type createTalkgroupRequest struct {
	SystemID    int64   `json:"systemId"`
	TalkgroupID int64   `json:"talkgroupId"`
	Label       *string `json:"label"`
	Name        *string `json:"name"`
	Frequency   *int64  `json:"frequency"`
	Led         *string `json:"led"`
	GroupID     *int64  `json:"groupId"`
	TagID       *int64  `json:"tagId"`
	Order       int64   `json:"order"`
}

func (r createTalkgroupRequest) toParams() db.CreateTalkgroupParams {
	return db.CreateTalkgroupParams{
		SystemID:    r.SystemID,
		TalkgroupID: r.TalkgroupID,
		Label:       ptrToNullStr(r.Label),
		Name:        ptrToNullStr(r.Name),
		Frequency:   ptrToNullInt(r.Frequency),
		Led:         ptrToNullStr(r.Led),
		GroupID:     ptrToNullInt(r.GroupID),
		TagID:       ptrToNullInt(r.TagID),
		Order:       r.Order,
	}
}

type updateTalkgroupRequest struct {
	TalkgroupID int64   `json:"talkgroupId"`
	Label       *string `json:"label"`
	Name        *string `json:"name"`
	Frequency   *int64  `json:"frequency"`
	Led         *string `json:"led"`
	GroupID     *int64  `json:"groupId"`
	TagID       *int64  `json:"tagId"`
	Order       int64   `json:"order"`
}

func (r updateTalkgroupRequest) toParams(id int64) db.UpdateTalkgroupParams {
	return db.UpdateTalkgroupParams{
		ID:          id,
		TalkgroupID: r.TalkgroupID,
		Label:       ptrToNullStr(r.Label),
		Name:        ptrToNullStr(r.Name),
		Frequency:   ptrToNullInt(r.Frequency),
		Led:         ptrToNullStr(r.Led),
		GroupID:     ptrToNullInt(r.GroupID),
		TagID:       ptrToNullInt(r.TagID),
		Order:       r.Order,
	}
}

// ── Unit requests ──

type createUnitRequest struct {
	SystemID int64   `json:"systemId"`
	UnitID   int64   `json:"unitId"`
	Label    *string `json:"label"`
	Order    int64   `json:"order"`
}

func (r createUnitRequest) toParams() db.CreateUnitParams {
	return db.CreateUnitParams{
		SystemID: r.SystemID,
		UnitID:   r.UnitID,
		Label:    ptrToNullStr(r.Label),
		Order:    r.Order,
	}
}

type updateUnitRequest struct {
	UnitID int64   `json:"unitId"`
	Label  *string `json:"label"`
	Order  int64   `json:"order"`
}

func (r updateUnitRequest) toParams(id int64) db.UpdateUnitParams {
	return db.UpdateUnitParams{
		ID:     id,
		UnitID: r.UnitID,
		Label:  ptrToNullStr(r.Label),
		Order:  r.Order,
	}
}

// ── API Key requests ──

type createAPIKeyRequest struct {
	Key         string  `json:"key"`
	Ident       *string `json:"ident"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson"`
	Order       int64   `json:"order"`
}

func (r createAPIKeyRequest) toParams() db.CreateAPIKeyParams {
	return db.CreateAPIKeyParams{
		Key:         r.Key,
		Ident:       ptrToNullStr(r.Ident),
		Disabled:    r.Disabled,
		SystemsJson: ptrToNullStr(r.SystemsJson),
		Order:       r.Order,
	}
}

type updateAPIKeyRequest struct {
	Key         string  `json:"key"`
	Ident       *string `json:"ident"`
	Disabled    int64   `json:"disabled"`
	SystemsJson *string `json:"systemsJson"`
	Order       int64   `json:"order"`
}

func (r updateAPIKeyRequest) toParams(id int64) db.UpdateAPIKeyParams {
	return db.UpdateAPIKeyParams{
		ID:          id,
		Key:         r.Key,
		Ident:       ptrToNullStr(r.Ident),
		Disabled:    r.Disabled,
		SystemsJson: ptrToNullStr(r.SystemsJson),
		Order:       r.Order,
	}
}

// ── Dirwatch requests ──

type createDirwatchRequest struct {
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
}

func (r createDirwatchRequest) toParams() db.CreateDirwatchParams {
	return db.CreateDirwatchParams{
		Directory:   r.Directory,
		Type:        r.Type,
		Mask:        ptrToNullStr(r.Mask),
		Extension:   ptrToNullStr(r.Extension),
		Frequency:   ptrToNullInt(r.Frequency),
		Delay:       ptrToNullInt(r.Delay),
		DeleteAfter: r.DeleteAfter,
		UsePolling:  r.UsePolling,
		Disabled:    r.Disabled,
		SystemID:    ptrToNullInt(r.SystemID),
		TalkgroupID: ptrToNullInt(r.TalkgroupID),
		Order:       r.Order,
	}
}

type updateDirwatchRequest struct {
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
}

func (r updateDirwatchRequest) toParams(id int64) db.UpdateDirwatchParams {
	return db.UpdateDirwatchParams{
		ID:          id,
		Directory:   r.Directory,
		Type:        r.Type,
		Mask:        ptrToNullStr(r.Mask),
		Extension:   ptrToNullStr(r.Extension),
		Frequency:   ptrToNullInt(r.Frequency),
		Delay:       ptrToNullInt(r.Delay),
		DeleteAfter: r.DeleteAfter,
		UsePolling:  r.UsePolling,
		Disabled:    r.Disabled,
		SystemID:    ptrToNullInt(r.SystemID),
		TalkgroupID: ptrToNullInt(r.TalkgroupID),
		Order:       r.Order,
	}
}

// ── Downstream requests ──

type createDownstreamRequest struct {
	Url         string  `json:"url"`
	ApiKey      string  `json:"apiKey"`
	SystemsJson *string `json:"systemsJson"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
}

func (r createDownstreamRequest) toParams() db.CreateDownstreamParams {
	return db.CreateDownstreamParams{
		Url:         r.Url,
		ApiKey:      r.ApiKey,
		SystemsJson: ptrToNullStr(r.SystemsJson),
		Disabled:    r.Disabled,
		Order:       r.Order,
	}
}

type updateDownstreamRequest struct {
	Url         string  `json:"url"`
	ApiKey      string  `json:"apiKey"`
	SystemsJson *string `json:"systemsJson"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
}

func (r updateDownstreamRequest) toParams(id int64) db.UpdateDownstreamParams {
	return db.UpdateDownstreamParams{
		ID:          id,
		Url:         r.Url,
		ApiKey:      r.ApiKey,
		SystemsJson: ptrToNullStr(r.SystemsJson),
		Disabled:    r.Disabled,
		Order:       r.Order,
	}
}

// ── Webhook requests ──

type createWebhookRequest struct {
	Url         string  `json:"url"`
	Type        string  `json:"type"`
	Secret      *string `json:"secret"`
	SystemsJson *string `json:"systemsJson"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
}

func (r createWebhookRequest) toParams() db.CreateWebhookParams {
	return db.CreateWebhookParams{
		Url:         r.Url,
		Type:        r.Type,
		Secret:      ptrToNullStr(r.Secret),
		SystemsJson: ptrToNullStr(r.SystemsJson),
		Disabled:    r.Disabled,
		Order:       r.Order,
	}
}

type updateWebhookRequest struct {
	Url         string  `json:"url"`
	Type        string  `json:"type"`
	Secret      *string `json:"secret"`
	SystemsJson *string `json:"systemsJson"`
	Disabled    int64   `json:"disabled"`
	Order       int64   `json:"order"`
}

func (r updateWebhookRequest) toParams(id int64) db.UpdateWebhookParams {
	return db.UpdateWebhookParams{
		ID:          id,
		Url:         r.Url,
		Type:        r.Type,
		Secret:      ptrToNullStr(r.Secret),
		SystemsJson: ptrToNullStr(r.SystemsJson),
		Disabled:    r.Disabled,
		Order:       r.Order,
	}
}
