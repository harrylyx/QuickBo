// Package cms talks to MacCMS provide/vod endpoints, both the JSON
// (type 1) and XML (type 0) shapes. The response body is sniffed, so a
// type 0 endpoint that actually returns JSON still parses.
package cms

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/harrylyx/QuickBo/internal/httpx"
	"github.com/harrylyx/QuickBo/internal/model"
)

// Client is one CMS endpoint.
type Client struct {
	base string
}

// New builds a client. base is the provide/vod URL, with or without a query.
func New(base string) *Client {
	return &Client{base: strings.TrimSpace(base)}
}

// Categories returns classes from ac=list. A failure here does not stop
// the caller from listing the latest page.
func (c *Client) Categories(ctx context.Context) ([]model.Category, error) {
	body, err := c.get(ctx, url.Values{"ac": {"list"}, "pg": {"1"}})
	if err != nil {
		return nil, err
	}
	cats, _, err := parseDocument(body)
	return cats, err
}

// List returns one page. cat is the type id; empty lists the latest page.
func (c *Client) List(ctx context.Context, cat string, page int) (model.Page, error) {
	if page <= 0 {
		page = 1
	}
	q := url.Values{"ac": {"videolist"}, "pg": {strconv.Itoa(page)}}
	if strings.TrimSpace(cat) != "" {
		q.Set("t", cat)
	}
	return c.page(ctx, q, page)
}

// Search searches by keyword.
func (c *Client) Search(ctx context.Context, keyword string, page int) (model.Page, error) {
	if page <= 0 {
		page = 1
	}
	q := url.Values{
		"ac": {"videolist"},
		"wd": {keyword},
		"pg": {strconv.Itoa(page)},
	}
	return c.page(ctx, q, page)
}

// Detail loads one title. id is the vod id.
func (c *Client) Detail(ctx context.Context, id string) (model.Detail, error) {
	body, err := c.get(ctx, url.Values{"ac": {"detail"}, "ids": {id}})
	if err != nil {
		return model.Detail{}, err
	}
	_, parsed, err := parseDocument(body)
	if err != nil {
		return model.Detail{}, err
	}
	if len(parsed.Page.Items) == 0 && len(parsed.Details) == 0 {
		return model.Detail{}, fmt.Errorf("详情为空")
	}
	if len(parsed.Details) > 0 {
		return parsed.Details[0], nil
	}
	it := parsed.Page.Items[0]
	return model.Detail{ID: it.ID, Title: it.Title, Pic: it.Pic, Remarks: it.Remarks, Year: it.Year}, nil
}

// pageResult carries both list items and full details from one document.
type pageResult struct {
	Page    model.Page
	Details []model.Detail
}

func (c *Client) page(ctx context.Context, q url.Values, page int) (model.Page, error) {
	body, err := c.get(ctx, q)
	if err != nil {
		return model.Page{}, err
	}
	_, parsed, err := parseDocument(body)
	if err != nil {
		return model.Page{}, err
	}
	if parsed.Page.Page == 0 {
		parsed.Page.Page = page
	}
	return parsed.Page, nil
}

func (c *Client) get(ctx context.Context, q url.Values) ([]byte, error) {
	u, err := url.Parse(c.base)
	if err != nil {
		return nil, fmt.Errorf("站点地址无效: %w", err)
	}
	exist := u.Query()
	for k, vs := range q {
		if len(vs) > 0 {
			exist.Set(k, vs[0])
		}
	}
	u.RawQuery = exist.Encode()
	body, _, err := httpx.Get(ctx, nil, u.String(), map[string]string{"User-Agent": httpx.TVBoxUA}, 8<<20)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func parseDocument(body []byte) ([]model.Category, pageResult, error) {
	body = stripJSONP(bytes.TrimSpace(bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})))
	if len(body) == 0 {
		return nil, pageResult{}, fmt.Errorf("站点返回空内容")
	}
	if body[0] == '<' {
		head := strings.ToLower(string(body[:min(len(body), 240)]))
		if strings.Contains(head, "<html") || strings.Contains(head, "<!doctype") {
			snip := body
			if len(snip) > 60 {
				snip = snip[:60]
			}
			return nil, pageResult{}, fmt.Errorf("站点返回的不是 JSON 或 XML 接口（开头为 %q）", strings.TrimSpace(string(snip)))
		}
		return parseXML(body)
	}
	if body[0] == '{' || body[0] == '[' {
		return parseJSON(body)
	}
	snip := body
	if len(snip) > 60 {
		snip = snip[:60]
	}
	return nil, pageResult{}, fmt.Errorf("站点返回的不是 JSON 或 XML（开头为 %q）", strings.TrimSpace(string(snip)))
}

func stripJSONP(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] == '{' || b[0] == '[' || b[0] == '<' {
		return b
	}
	i := bytes.IndexByte(b, '(')
	if i < 0 || i > 64 || b[len(b)-1] != ')' {
		return b
	}
	inner := bytes.TrimSpace(b[i+1 : len(b)-1])
	if len(inner) > 0 && (inner[0] == '{' || inner[0] == '[') {
		return inner
	}
	return b
}

type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexID(strings.TrimSpace(s))
		return nil
	}
	*f = flexID(string(b))
	return nil
}

func (f flexID) String() string { return string(f) }

type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		n, _ := strconv.Atoi(strings.TrimSpace(s))
		*f = flexInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	var fl float64
	if err := json.Unmarshal(b, &fl); err != nil {
		return err
	}
	*f = flexInt(fl)
	return nil
}

type cmsVideo struct {
	VodID       flexID `json:"vod_id"`
	VodName     string `json:"vod_name"`
	VodPic      string `json:"vod_pic"`
	TypeName    string `json:"type_name"`
	VodRemarks  string `json:"vod_remarks"`
	VodYear     string `json:"vod_year"`
	VodArea     string `json:"vod_area"`
	VodContent  string `json:"vod_content"`
	VodPlayFrom string `json:"vod_play_from"`
	VodPlayURL  string `json:"vod_play_url"`
}

type cmsClass struct {
	TypeID   flexID `json:"type_id"`
	TypeName string `json:"type_name"`
}

type cmsResp struct {
	Code      flexInt    `json:"code"`
	Msg       string     `json:"msg"`
	List      []cmsVideo `json:"list"`
	Class     []cmsClass `json:"class"`
	Page      flexInt    `json:"page"`
	PageCount flexInt    `json:"pagecount"`
	Total     flexInt    `json:"total"`
}

func parseJSON(body []byte) ([]model.Category, pageResult, error) {
	var resp cmsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, pageResult{}, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	if len(resp.List) == 0 && len(resp.Class) == 0 && resp.Code != 1 && resp.Code != 200 {
		msg := strings.TrimSpace(resp.Msg)
		if msg == "" {
			msg = fmt.Sprintf("站点返回错误码 %d", resp.Code)
		}
		return nil, pageResult{}, fmt.Errorf("%s", msg)
	}
	cats := make([]model.Category, 0, len(resp.Class))
	for _, c := range resp.Class {
		if strings.TrimSpace(c.TypeName) == "" {
			continue
		}
		cats = append(cats, model.Category{ID: c.TypeID.String(), Name: strings.TrimSpace(c.TypeName)})
	}
	items := make([]model.Item, 0, len(resp.List))
	details := make([]model.Detail, 0, len(resp.List))
	for _, v := range resp.List {
		title := strings.TrimSpace(v.VodName)
		if title == "" {
			continue
		}
		id := v.VodID.String()
		items = append(items, model.Item{
			ID: id, Title: title, Pic: strings.TrimSpace(v.VodPic),
			Remarks: strings.TrimSpace(v.VodRemarks), Year: strings.TrimSpace(v.VodYear),
		})
		details = append(details, model.Detail{
			ID: id, Title: title, Pic: strings.TrimSpace(v.VodPic),
			Remarks: strings.TrimSpace(v.VodRemarks), Year: strings.TrimSpace(v.VodYear),
			Area: strings.TrimSpace(v.VodArea), TypeName: strings.TrimSpace(v.TypeName),
			Content: model.StripTags(v.VodContent),
			Lines:   model.ParsePlay(v.VodPlayFrom, v.VodPlayURL),
		})
	}
	return cats, pageResult{
		Page: model.Page{
			Items: items, Page: int(resp.Page), PageCount: int(resp.PageCount), Total: int(resp.Total),
		},
		Details: details,
	}, nil
}

type xmlClass struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

type xmlDD struct {
	Flag string `xml:"flag,attr"`
	Body string `xml:",chardata"`
}

type xmlVideo struct {
	ID      string  `xml:"id"`
	TID     string  `xml:"tid"`
	Name    string  `xml:"name"`
	Pic     string  `xml:"pic"`
	Note    string  `xml:"note"`
	Type    string  `xml:"type"`
	Year    string  `xml:"year"`
	Area    string  `xml:"area"`
	Des     string  `xml:"des"`
	Lang    string  `xml:"lang"`
	VodName string  `xml:"vod_name"`
	VodPic  string  `xml:"vod_pic"`
	VodID   string  `xml:"vod_id"`
	Content string  `xml:"vod_content"`
	From    string  `xml:"vod_play_from"`
	Play    string  `xml:"vod_play_url"`
	DL      []xmlDD `xml:"dl>dd"`
}

type xmlList struct {
	Page      int        `xml:"page,attr"`
	PageCount int        `xml:"pagecount,attr"`
	Total     int        `xml:"recordcount,attr"`
	Videos    []xmlVideo `xml:"video"`
}

type xmlDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Classes []xmlClass `xml:"class>ty"`
	Lists   []xmlList  `xml:"list"`
}

func parseXML(body []byte) ([]model.Category, pageResult, error) {
	var doc xmlDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, pageResult{}, fmt.Errorf("解析 XML 失败: %w", err)
	}
	cats := make([]model.Category, 0, len(doc.Classes))
	for _, c := range doc.Classes {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			continue
		}
		cats = append(cats, model.Category{ID: strings.TrimSpace(c.ID), Name: name})
	}
	var items []model.Item
	var details []model.Detail
	page, pageCount, total := 0, 0, 0
	for _, list := range doc.Lists {
		if list.Page > 0 {
			page = list.Page
		}
		if list.PageCount > 0 {
			pageCount = list.PageCount
		}
		if list.Total > 0 {
			total = list.Total
		}
		for _, v := range list.Videos {
			title := strings.TrimSpace(v.Name)
			if title == "" {
				title = strings.TrimSpace(v.VodName)
			}
			if title == "" {
				continue
			}
			id := strings.TrimSpace(v.ID)
			if id == "" {
				id = strings.TrimSpace(v.VodID)
			}
			pic := strings.TrimSpace(v.Pic)
			if pic == "" {
				pic = strings.TrimSpace(v.VodPic)
			}
			items = append(items, model.Item{ID: id, Title: title, Pic: pic, Remarks: strings.TrimSpace(v.Note), Year: strings.TrimSpace(v.Year)})
			from := v.From
			play := v.Play
			if play == "" && len(v.DL) > 0 {
				var froms, plays []string
				for _, dd := range v.DL {
					flag := strings.TrimSpace(dd.Flag)
					if flag == "" {
						flag = "默认"
					}
					body := strings.TrimSpace(dd.Body)
					if body == "" {
						continue
					}
					froms = append(froms, flag)
					plays = append(plays, body)
				}
				from = strings.Join(froms, "$$$")
				play = strings.Join(plays, "$$$")
			}
			content := v.Des
			if strings.TrimSpace(content) == "" {
				content = v.Content
			}
			details = append(details, model.Detail{
				ID: id, Title: title, Pic: pic, Remarks: strings.TrimSpace(v.Note),
				Year: strings.TrimSpace(v.Year), Area: strings.TrimSpace(v.Area),
				TypeName: strings.TrimSpace(v.Type), Content: model.StripTags(content),
				Lines: model.ParsePlay(from, play),
			})
		}
	}
	return cats, pageResult{
		Page:    model.Page{Items: items, Page: page, PageCount: pageCount, Total: total},
		Details: details,
	}, nil
}
