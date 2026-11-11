package app

import (
	"context"
	"strings"
	"time"

	"github.com/harrylyx/QuickBo/internal/httpx"
	"github.com/harrylyx/QuickBo/internal/live"
	"github.com/harrylyx/QuickBo/internal/player"
)

const (
	// A healthy stream is trusted for half an hour; a failed one is skipped for
	// five minutes so a channel the user keeps clicking is not re-probed and
	// re-opened on every attempt.
	liveOKTTL   = 30 * time.Minute
	liveFailTTL = 5 * time.Minute
)

// liveHealthState is the cached result of probing one live stream.
type liveHealthState struct {
	ok bool
	at time.Time
}

// startChannel opens one live channel, probing its stream first. A dead
// channel is swapped for the next one in the group (see switchChannel) instead
// of opening an mpv window that just shows an error.
func (a *App) startChannel(ch live.Channel) {
	a.playLine = ch.Group
	a.playTitle = ch.Name
	a.fromLive = true
	a.screen = screenLive
	a.playLoading = true
	a.playErr = ""
	a.closePlayer()
	headers := map[string]string{"User-Agent": httpx.TVBoxUA}
	raw := ch.URL
	name := ch.Name
	id := "live:" + strings.TrimSpace(ch.URL)
	if a.liveHealth == nil {
		a.liveHealth = map[string]liveHealthState{}
	}
	if st, ok := a.liveHealth[id]; ok {
		if st.ok && time.Since(st.at) < liveOKTTL {
			a.playLoading = false
			a.startPlay(name, raw, headers, true)
			a.screen = screenLive
			a.fromLive = true
			return
		}
		if !st.ok && time.Since(st.at) < liveFailTTL {
			a.playLoading = false
			a.switchChannel(ch)
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if !player.IsDirect(raw) {
			if u, ok := player.Unwrap(ctx, raw, headers); ok && u != "" {
				raw = u
			}
		}
		// Read a few KB and abort: any bytes plus a 2xx status means the
		// stream is serving; a connect error or timeout means switch lines.
		pctx, pcancel := context.WithTimeout(context.Background(), 6*time.Second)
		_, err := httpx.Probe(pctx, nil, raw, headers, 4<<10)
		pcancel()
		ok := err == nil
		a.post(func() {
			a.playLoading = false
			a.liveHealth[id] = liveHealthState{ok: ok, at: time.Now()}
			if !ok {
				a.switchChannel(ch)
				return
			}
			a.startPlay(name, raw, headers, true)
			a.screen = screenLive
			a.fromLive = true
		})
	}()
}

// switchChannel moves to the next channel after a failure. liveTries bounds
// the walk so a dead group cannot loop forever.
func (a *App) switchChannel(ch live.Channel) {
	a.liveTries++
	chs := a.channels
	if a.liveTries > len(chs) {
		a.playErr = "没有可用的直播线路"
		return
	}
	idx := -1
	for i, c := range chs {
		if c.URL == ch.URL && c.Name == ch.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		a.playErr = "这条线路不可用"
		return
	}
	// Prefer the next channel in the same group, then the next channel overall.
	for j := 1; j < len(chs); j++ {
		n := (idx + j) % len(chs)
		if chs[n].Group == ch.Group {
			a.startChannel(chs[n])
			return
		}
	}
	a.startChannel(chs[(idx+1)%len(chs)])
}
