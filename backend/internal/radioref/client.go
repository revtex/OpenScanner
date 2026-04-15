// Package radioref provides a client for the RadioReference.com SOAP API.
package radioref

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	soapEndpoint = "https://api.radioreference.com/soap2/"
	soapNS       = "http://api.radioreference.com/soap2"
	apiVersion   = "latest"
	apiStyle     = "rpc"
)

// Client calls the RadioReference SOAP API.
type Client struct {
	httpClient *http.Client
	appKey     string
}

// NewClient creates a new RadioReference API client.
func NewClient(appKey string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		appKey: appKey,
	}
}

// ── Public response types ──

// Country from getCountryList.
type Country struct {
	ID   int    `json:"coid"`
	Name string `json:"countryName"`
	Code string `json:"countryCode"`
}

// State from getCountryInfo → stateList.
type State struct {
	ID   int    `json:"stid"`
	Name string `json:"stateName"`
	Code string `json:"stateCode"`
}

// County from getStateInfo → countyList.
type County struct {
	ID     int    `json:"ctid"`
	Name   string `json:"countyName"`
	Header string `json:"countyHeader"`
}

// System from getCountyInfo → trsList.
type System struct {
	SID  int    `json:"sid"`
	Name string `json:"sName"`
	City string `json:"sCity"`
}

// Talkgroup from getTrsTalkgroups.
type Talkgroup struct {
	Dec   int    `json:"tgDec"`
	Alpha string `json:"tgAlpha"`
	Descr string `json:"tgDescr"`
	Mode  string `json:"tgMode"`
	Sort  int    `json:"tgSort"`
	CatID int    `json:"tgCid"`
	Tags  []Tag  `json:"tags"`
}

// TalkgroupCat from getTrsTalkgroupCats.
type TalkgroupCat struct {
	CID  int    `json:"tgCid"`
	Name string `json:"tgCname"`
	Sort int    `json:"tgSort"`
}

// Tag within a Talkgroup.
type Tag struct {
	ID   int    `json:"tagId"`
	Name string `json:"tagDescr"`
}

// UserInfo from getUserData.
type UserInfo struct {
	Username  string `json:"username"`
	ExpiresAt string `json:"subExpireDate"`
}

// ── API methods ──

// GetUserData validates credentials and returns user info.
func (c *Client) GetUserData(ctx context.Context, username, password string) (*UserInfo, error) {
	body := c.soapEnvelope("getUserData", c.authInfoXML(username, password))
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					Username  string `xml:"username"`
					ExpiresAt string `xml:"subExpireDate"`
				} `xml:"return"`
			} `xml:"getUserDataResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getUserData", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	return &UserInfo{
		Username:  resp.Body.Response.Return.Username,
		ExpiresAt: resp.Body.Response.Return.ExpiresAt,
	}, nil
}

// GetCountryList returns all countries. Does not require auth.
func (c *Client) GetCountryList(ctx context.Context) ([]Country, error) {
	body := c.soapEnvelope("getCountryList", "")
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					Items []struct {
						ID   int    `xml:"coid"`
						Name string `xml:"countryName"`
						Code string `xml:"countryCode"`
					} `xml:"item"`
				} `xml:"return"`
			} `xml:"getCountryListResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getCountryList", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	items := resp.Body.Response.Return.Items
	countries := make([]Country, len(items))
	for i, it := range items {
		countries[i] = Country{ID: it.ID, Name: it.Name, Code: it.Code}
	}
	return countries, nil
}

// GetStates returns the states/provinces for a country.
func (c *Client) GetStates(ctx context.Context, countryID int, username, password string) ([]State, error) {
	params := fmt.Sprintf(`<coid xsi:type="xsd:int">%d</coid>`, countryID) + c.authInfoXML(username, password)
	body := c.soapEnvelope("getCountryInfo", params)
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					StateList struct {
						Items []struct {
							ID   int    `xml:"stid"`
							Name string `xml:"stateName"`
							Code string `xml:"stateCode"`
						} `xml:"item"`
					} `xml:"stateList"`
				} `xml:"return"`
			} `xml:"getCountryInfoResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getCountryInfo", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	items := resp.Body.Response.Return.StateList.Items
	states := make([]State, len(items))
	for i, it := range items {
		states[i] = State{ID: it.ID, Name: it.Name, Code: it.Code}
	}
	return states, nil
}

// GetCounties returns the counties for a state.
func (c *Client) GetCounties(ctx context.Context, stateID int, username, password string) ([]County, error) {
	params := fmt.Sprintf(`<stid xsi:type="xsd:int">%d</stid>`, stateID) + c.authInfoXML(username, password)
	body := c.soapEnvelope("getStateInfo", params)
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					CountyList struct {
						Items []struct {
							ID     int    `xml:"ctid"`
							Name   string `xml:"countyName"`
							Header string `xml:"countyHeader"`
						} `xml:"item"`
					} `xml:"countyList"`
				} `xml:"return"`
			} `xml:"getStateInfoResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getStateInfo", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	items := resp.Body.Response.Return.CountyList.Items
	counties := make([]County, len(items))
	for i, it := range items {
		counties[i] = County{ID: it.ID, Name: it.Name, Header: it.Header}
	}
	return counties, nil
}

// GetSystems returns the trunked radio systems for a county.
func (c *Client) GetSystems(ctx context.Context, countyID int, username, password string) ([]System, error) {
	params := fmt.Sprintf(`<ctid xsi:type="xsd:int">%d</ctid>`, countyID) + c.authInfoXML(username, password)
	body := c.soapEnvelope("getCountyInfo", params)
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					TrsList struct {
						Items []struct {
							SID  int    `xml:"sid"`
							Name string `xml:"sName"`
							City string `xml:"sCity"`
						} `xml:"item"`
					} `xml:"trsList"`
				} `xml:"return"`
			} `xml:"getCountyInfoResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getCountyInfo", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	items := resp.Body.Response.Return.TrsList.Items
	systems := make([]System, len(items))
	for i, it := range items {
		systems[i] = System{SID: it.SID, Name: it.Name, City: it.City}
	}
	return systems, nil
}

// GetTrsTalkgroups returns all talkgroups for a trunked system.
func (c *Client) GetTrsTalkgroups(ctx context.Context, sid int, username, password string) ([]Talkgroup, error) {
	params := fmt.Sprintf(`<sid xsi:type="xsd:int">%d</sid>`, sid) +
		`<tgCid xsi:type="xsd:int">0</tgCid>` +
		`<tgTag xsi:type="xsd:int">0</tgTag>` +
		`<tgDec xsi:type="xsd:int">0</tgDec>` +
		c.authInfoXML(username, password)
	body := c.soapEnvelope("getTrsTalkgroups", params)
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					Items []struct {
						Dec   int    `xml:"tgDec"`
						Alpha string `xml:"tgAlpha"`
						Descr string `xml:"tgDescr"`
						Mode  string `xml:"tgMode"`
						Sort  int    `xml:"tgSort"`
						CatID int    `xml:"tgCid"`
						Tags  struct {
							Items []struct {
								ID   int    `xml:"tagId"`
								Name string `xml:"tagDescr"`
							} `xml:"item"`
						} `xml:"tags"`
					} `xml:"item"`
				} `xml:"return"`
			} `xml:"getTrsTalkgroupsResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getTrsTalkgroups", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	items := resp.Body.Response.Return.Items
	tgs := make([]Talkgroup, len(items))
	for i, it := range items {
		tags := make([]Tag, len(it.Tags.Items))
		for j, t := range it.Tags.Items {
			tags[j] = Tag{ID: t.ID, Name: t.Name}
		}
		tgs[i] = Talkgroup{
			Dec:   it.Dec,
			Alpha: it.Alpha,
			Descr: it.Descr,
			Mode:  it.Mode,
			Sort:  it.Sort,
			CatID: it.CatID,
			Tags:  tags,
		}
	}
	return tgs, nil
}

// GetTrsTalkgroupCats returns talkgroup categories for a system.
func (c *Client) GetTrsTalkgroupCats(ctx context.Context, sid int, username, password string) ([]TalkgroupCat, error) {
	params := fmt.Sprintf(`<sid xsi:type="xsd:int">%d</sid>`, sid) + c.authInfoXML(username, password)
	body := c.soapEnvelope("getTrsTalkgroupCats", params)
	var resp struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Response struct {
				Return struct {
					Items []struct {
						CID  int    `xml:"tgCid"`
						Name string `xml:"tgCname"`
						Sort int    `xml:"tgSort"`
					} `xml:"item"`
				} `xml:"return"`
			} `xml:"getTrsTalkgroupCatsResponse"`
			Fault *soapFault `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := c.doSOAP(ctx, "getTrsTalkgroupCats", body, &resp); err != nil {
		return nil, err
	}
	if resp.Body.Fault != nil {
		return nil, resp.Body.Fault
	}
	items := resp.Body.Response.Return.Items
	cats := make([]TalkgroupCat, len(items))
	for i, it := range items {
		cats[i] = TalkgroupCat{CID: it.CID, Name: it.Name, Sort: it.Sort}
	}
	return cats, nil
}

// ── SOAP helpers ──

type soapFault struct {
	Code   string `xml:"faultcode"`
	String string `xml:"faultstring"`
}

func (f *soapFault) Error() string {
	return fmt.Sprintf("SOAP fault %s: %s", f.Code, f.String)
}

func (c *Client) authInfoXML(username, password string) string {
	return fmt.Sprintf(`<authInfo xsi:type="ns1:authInfo">`+
		`<username xsi:type="xsd:string">%s</username>`+
		`<password xsi:type="xsd:string">%s</password>`+
		`<appKey xsi:type="xsd:string">%s</appKey>`+
		`<version xsi:type="xsd:string">%s</version>`+
		`<style xsi:type="xsd:string">%s</style>`+
		`</authInfo>`,
		xmlEscape(username), xmlEscape(password), xmlEscape(c.appKey), apiVersion, apiStyle)
}

func (c *Client) soapEnvelope(method, innerXML string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<SOAP-ENV:Envelope ` +
		`xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`xmlns:ns1="` + soapNS + `" ` +
		`xmlns:xsd="http://www.w3.org/2001/XMLSchema" ` +
		`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ` +
		`xmlns:SOAP-ENC="http://schemas.xmlsoap.org/soap/encoding/" ` +
		`SOAP-ENV:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<SOAP-ENV:Body>` +
		`<ns1:` + method + `>` +
		innerXML +
		`</ns1:` + method + `>` +
		`</SOAP-ENV:Body>` +
		`</SOAP-ENV:Envelope>`
}

func (c *Client) doSOAP(ctx context.Context, action string, body string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, soapEndpoint, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", soapNS+"#"+action)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB max
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		return fmt.Errorf("HTTP %d from RadioReference API", resp.StatusCode)
	}

	if err := xml.Unmarshal(data, result); err != nil {
		return fmt.Errorf("parse SOAP response: %w", err)
	}
	return nil
}

func xmlEscape(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return s
	}
	return b.String()
}

// ErrNoAppKey is returned when the RadioReference app key is not configured.
var ErrNoAppKey = errors.New("RadioReference API app key is not configured")
