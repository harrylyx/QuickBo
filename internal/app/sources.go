package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/httpx"
	"github.com/harrylyx/QuickBo/internal/source"
	"golang.org/x/net/idna"
)

// sourceStat is the last speed test of one site.
type sourceStat struct {
	Delay int    `json:"delay"`
	IP    string `json:"ip,omitempty"`
	Place string `json:"place,omitempty"`
	Err   string `json:"err,omitempty"`
}

// healthTier buckets a speed test into a short label for the summary bar and
// the health grouping. It is deliberately separate from statColor, which only
// chooses a text color.
func healthTier(st sourceStat) string {
	switch {
	case st.Err != "" || st.Delay < 0:
		return "失败"
	case st.Delay == 0:
		return "未测"
	case st.Delay < 200:
		return "优秀"
	case st.Delay < 500:
		return "良好"
	case st.Delay < 1000:
		return "一般"
	default:
		return "较慢"
	}
}

// healthRank orders tiers from fastest to worst; unknown sorts after known.
func healthRank(st sourceStat) int {
	switch {
	case st.Err != "" || st.Delay < 0:
		return 5
	case st.Delay == 0:
		return 4
	case st.Delay < 200:
		return 0
	case st.Delay < 500:
		return 1
	case st.Delay < 1000:
		return 2
	default:
		return 3
	}
}

// probeableSites lists the sources worth a speed test: everything kept and
// listed in the management view except unsupported entries.
func (a *App) probeableSites() []source.Source {
	var out []source.Source
	for _, s := range a.allSites {
		m := s.Meta()
		if a.hidden[sourceHideID(s)] || a.blacklisted(m.Name) || a.disabled[sourceHideID(s)] || m.Level == config.LevelNo {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (a *App) probeAllSources() {
	keys := make([]string, 0, len(a.probeableSites()))
	for _, s := range a.probeableSites() {
		keys = append(keys, s.Meta().Key)
	}
	a.probeSources(keys)
}

// probeSources sweeps a chosen set of sources and reports one progress bar.
func (a *App) probeSources(keys []string) {
	if len(keys) == 0 {
		return
	}
	a.probingTotal = len(keys)
	a.probingDone = 0
	a.probingActive = true
	for _, k := range keys {
		a.probeSource(k)
	}
}

func (a *App) probeSource(key string) {
	if a.catalog == nil {
		return
	}
	src := a.catalog.Find(key)
	if src == nil {
		src = a.findAll(key)
	}
	if src == nil {
		return
	}
	api := src.Meta().API
	if a.stats == nil {
		a.stats = map[string]sourceStat{}
	}
	cur := a.stats[key]
	cur.Err = ""
	cur.Delay = 0
	a.stats[key] = cur
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		st := measureSource(ctx, api)
		a.post(func() {
			if a.stats == nil {
				a.stats = map[string]sourceStat{}
			}
			a.stats[key] = st
			a.finishProbe()
			if a.screen == screenDetail {
				a.sortOffers()
			}
		})
	}()
}

// finishProbe advances the batch progress and persists the speed results once
// the whole sweep (or a lone probe) is complete.
func (a *App) finishProbe() {
	if a.probingActive {
		a.probingDone++
		if a.probingDone >= a.probingTotal {
			a.probingActive = false
		}
	}
	if !a.probingActive {
		a.saveSettings()
	}
}

func measureSource(ctx context.Context, raw string) sourceStat {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return sourceStat{Delay: -1, Err: "没有可测的地址"}
	}
	host := u.Hostname()
	ip := resolvePublicIP(ctx, host)
	start := time.Now()
	_, _, err = httpx.Get(ctx, nil, raw, nil, 32<<10)
	delay := int(time.Since(start).Milliseconds())
	if delay < 1 {
		delay = 1
	}
	st := sourceStat{Delay: delay}
	if ip != "" {
		st.IP = ip
		st.Place = placeOf(ctx, ip)
	}
	if err != nil {
		st.Delay = -1
		st.Err = "连不上"
	}
	return st
}

// benchmarkNet is the RFC 2544 range. Clash and sing-box fake-ip use it,
// so the system resolver answers 198.18.x.x for every hostname.
var benchmarkNet = func() *net.IPNet {
	_, n, err := net.ParseCIDR("198.18.0.0/15")
	if err != nil {
		panic(err)
	}
	return n
}()

func routableIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
		if benchmarkNet.Contains(ip) {
			return false
		}
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast()
}

// resolvePublicIP asks a public DNS-over-HTTPS resolver. The system
// resolver is not used: on this machine it returns the proxy's fake address.
func resolvePublicIP(ctx context.Context, host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		if routableIP(ip) {
			return ip.String()
		}
		return ""
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil && ascii != "" {
		host = ascii
	}
	for _, kind := range []string{"1", "28"} {
		q := "https://dns.alidns.com/resolve?name=" + url.QueryEscape(host) + "&type=" + kind
		body, _, err := httpx.Get(ctx, nil, q, map[string]string{"Accept": "application/dns-json"}, 1<<20)
		if err != nil {
			continue
		}
		if ip := firstAnswerIP(body); ip != "" {
			return ip
		}
	}
	return ""
}

func firstAnswerIP(body []byte) string {
	var resp struct {
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return ""
	}
	for _, a := range resp.Answer {
		if a.Type != 1 && a.Type != 28 {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(a.Data))
		if routableIP(ip) {
			return ip.String()
		}
	}
	return ""
}

func placeOf(ctx context.Context, ip string) string {
	parsed := net.ParseIP(ip)
	if !routableIP(parsed) {
		return ""
	}
	body, _, err := httpx.Get(ctx, nil, "https://ipwho.is/"+ip+"?lang=zh", nil, 1<<20)
	if err != nil {
		return ""
	}
	var who struct {
		Success bool   `json:"success"`
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
	}
	if json.Unmarshal(body, &who) != nil || !who.Success {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, p := range []string{who.Country, who.Region, who.City} {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(parts) > 0 && parts[len(parts)-1] == p {
			continue
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " ")
}

func (a *App) statLine(key string) string {
	st := a.stats[key]
	switch {
	case st.Delay > 0:
		line := fmt.Sprintf("%d ms", st.Delay)
		if st.IP != "" {
			line += "  ·  " + st.IP
		}
		if st.Place != "" {
			line += "  ·  " + st.Place
		}
		return line
	case st.Delay < 0:
		line := "失败"
		if st.Err != "" {
			line = st.Err
		}
		if st.IP != "" {
			line += "  ·  " + st.IP
		}
		if st.Place != "" {
			line += "  ·  " + st.Place
		}
		return line
	default:
		return "未测速"
	}
}
