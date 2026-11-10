package drpy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/harrylyx/mybox/internal/model"
)

func parseList(body, spec, base string) []model.Item {
	spec = strings.TrimSpace(spec)
	if spec == "" || strings.TrimSpace(body) == "" {
		return nil
	}
	parts := splitSemi(spec)
	if len(parts) == 0 {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(parts[0]), "json:") {
		return parseJSONList(body, parts, base)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil
	}
	nodes := queryAll(doc.Selection, parts[0])
	if nodes == nil || nodes.Length() == 0 {
		return nil
	}
	var out []model.Item
	nodes.Each(func(_ int, s *goquery.Selection) {
		title := field(s, part(parts, 1))
		pic := field(s, part(parts, 2))
		remarks := field(s, part(parts, 3))
		link := field(s, part(parts, 4))
		if link == "" {
			link, _ = s.Find("a").Attr("href")
		}
		if title == "" {
			title = strings.TrimSpace(s.Text())
		}
		link = absURL(base, link)
		if title == "" || link == "" {
			return
		}
		out = append(out, model.Item{ID: link, Title: oneLine(title), Pic: absURL(base, pic), Remarks: oneLine(remarks)})
	})
	return out
}

func parseDetail(body string, spec any, base string) model.Detail {
	var d model.Detail
	m, _ := spec.(map[string]any)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return d
	}
	if m == nil {
		if t := strings.TrimSpace(doc.Find("h1").First().Text()); t != "" {
			d.Title = oneLine(t)
		}
		return d
	}
	d.Title = oneLine(field(doc.Selection, asString(m["title"])))
	d.Pic = absURL(base, field(doc.Selection, asString(firstVal(m["img"], m["pic"]))))
	d.Remarks = oneLine(field(doc.Selection, asString(m["desc"])))
	d.Content = oneLine(field(doc.Selection, asString(m["content"])))
	tabsSel := asString(m["tabs"])
	listsSpec := asString(m["lists"])
	var tabNames []string
	if tabsSel != "" {
		if nodes := queryAll(doc.Selection, tabsSel); nodes != nil {
			nodes.Each(func(_ int, s *goquery.Selection) {
				if name := oneLine(s.Text()); name != "" {
					tabNames = append(tabNames, name)
				}
			})
		}
	}
	if listsSpec == "" {
		return d
	}
	if len(tabNames) == 0 {
		eps := links(doc, listsSpec, base)
		if len(eps) > 0 {
			d.Lines = []model.Line{{Name: "播放", Episodes: eps}}
		}
		return d
	}
	for i, name := range tabNames {
		spec := strings.ReplaceAll(listsSpec, "#id", strconv.Itoa(i))
		spec = strings.ReplaceAll(spec, "{id}", strconv.Itoa(i))
		eps := links(doc, spec, base)
		if len(eps) == 0 {
			continue
		}
		d.Lines = append(d.Lines, model.Line{Name: name, Episodes: eps})
	}
	return d
}

func links(doc *goquery.Document, spec, base string) []model.Episode {
	nodes := queryAll(doc.Selection, spec)
	if nodes == nil {
		return nil
	}
	var out []model.Episode
	nodes.Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if href == "" {
			href, _ = s.Find("a").Attr("href")
		}
		name := oneLine(s.Text())
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return
		}
		if name == "" {
			name = fmt.Sprintf("第%d集", len(out)+1)
		}
		out = append(out, model.Episode{Name: name, URL: absURL(base, href)})
	})
	return out
}

func parseJSONList(body string, parts []string, base string) []model.Item {
	var v any
	if json.Unmarshal([]byte(strings.TrimSpace(body)), &v) != nil {
		return nil
	}
	path := strings.TrimPrefix(parts[0], "json:")
	path = strings.TrimPrefix(path, "JSON:")
	arr := walk(v, path)
	list, _ := arr.([]any)
	var out []model.Item
	for _, item := range list {
		im, _ := item.(map[string]any)
		if im == nil {
			continue
		}
		title := asString(im[part(parts, 1)])
		pic := asString(im[part(parts, 2)])
		remarks := asString(im[part(parts, 3)])
		id := asString(im[part(parts, 4)])
		if title == "" || id == "" {
			continue
		}
		out = append(out, model.Item{ID: absIfURL(base, id), Title: title, Pic: absIfURL(base, pic), Remarks: remarks})
	}
	return out
}

func walk(v any, path string) any {
	cur := v
	path = strings.Trim(path, ".")
	if path == "" {
		return cur
	}
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func field(root *goquery.Selection, spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" || root == nil {
		return ""
	}
	for _, alt := range strings.Split(spec, "||") {
		if v := fieldOne(root, strings.TrimSpace(alt)); v != "" {
			return v
		}
	}
	return ""
}

func fieldOne(root *goquery.Selection, spec string) string {
	if spec == "" {
		return ""
	}
	sel, attr, has := strings.Cut(spec, "&&")
	nodes := queryAll(root, strings.TrimSpace(sel))
	if nodes == nil || nodes.Length() == 0 {
		if strings.TrimSpace(sel) == "" || sel == "*" {
			nodes = root
		} else {
			return ""
		}
	}
	node := nodes.First()
	if !has || strings.EqualFold(attr, "Text") {
		return strings.TrimSpace(node.Text())
	}
	if strings.EqualFold(attr, "Html") {
		h, _ := node.Html()
		return strings.TrimSpace(h)
	}
	if v, ok := node.Attr(attr); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func queryAll(root *goquery.Selection, spec string) *goquery.Selection {
	spec = strings.TrimSpace(spec)
	if root == nil {
		return nil
	}
	if spec == "" || spec == "*" {
		return root
	}
	if i := strings.Index(spec, ":eq("); i >= 0 {
		base := strings.TrimSpace(spec[:i])
		rest := spec[i+4:]
		nStr, after, _ := strings.Cut(rest, ")")
		n, _ := strconv.Atoi(strings.TrimSpace(nStr))
		var s *goquery.Selection
		if base == "" || base == "*" {
			s = root
		} else {
			s = safeFind(root, base)
		}
		if s == nil {
			return nil
		}
		s = s.Eq(n)
		after = strings.TrimSpace(after)
		if after == "" {
			return s
		}
		return queryAll(s, after)
	}
	return safeFind(root, spec)
}

func safeFind(root *goquery.Selection, sel string) (out *goquery.Selection) {
	defer func() { _ = recover() }()
	return root.Find(sel)
}

func splitSemi(s string) []string {
	return strings.Split(s, ";")
}

func part(parts []string, i int) string {
	if i < 0 || i >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[i])
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func absIfURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.Contains(ref, "://") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") || strings.HasPrefix(ref, "//") {
		return absURL(base, ref)
	}
	return ref
}

var mediaURLRe = regexp.MustCompile(`(?i)https?://[^"'\\\s<>]+(?:\.m3u8|\.mp4|\.flv)(?:\?[^"'\\\s<>]*)?`)

func findMedia(page string) string { return mediaURLRe.FindString(page) }
