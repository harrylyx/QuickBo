package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/player"
	"github.com/harrylyx/QuickBo/internal/source"
)

// offerProbe is what checking one source's lines found.
type offerProbe struct {
	w, h int
	// unplayable is set only when every checked line failed for a reason
	// that is not a timeout; reason says why, in the first line's terms.
	unplayable bool
	reason     string
}

// measureOffer checks one source: each play line contributes its first
// episode, resolved to a stream as playback would, and the reads run
// together. The largest picture wins. A line whose size cannot be read is
// still playable if its address answers with media (player.Check).
func (a *App) measureOffer(ctx context.Context, src source.Source, d model.Detail, parses []config.ParseItem) offerProbe {
	lines := d.Lines
	if len(lines) > 8 {
		lines = lines[:8]
	}
	type hit struct {
		w, h     int
		checked  bool // the line had an episode to check
		ok       bool
		timedOut bool
		reason   string
	}
	hits := make([]hit, len(lines))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, ln := range lines {
		if len(ln.Episodes) == 0 {
			continue
		}
		hits[i].checked = true
		wg.Add(1)
		go func(i int, name string, ep model.Episode) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				hits[i].timedOut = true
				return
			}
			pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			pb, err := src.Resolve(pctx, name, ep)
			if err != nil || strings.TrimSpace(pb.URL) == "" {
				if pctx.Err() != nil {
					hits[i].timedOut = true
				} else {
					hits[i].reason = "解析播放地址失败"
				}
				return
			}
			if pb.NeedsParse || !player.IsDirect(pb.URL) {
				if u, hdr, ok := a.applyParsesRanked(pctx, pb.URL, parses); ok {
					pb.URL = u
					pb.Headers = mergeHeaders(pb.Headers, hdr)
				}
			}
			if !player.IsDirect(pb.URL) {
				if u, ok := player.Unwrap(pctx, pb.URL, pb.Headers); ok && u != "" {
					pb.URL = u
				}
			}
			if w, h := player.Measure(pctx, pb.URL, pb.Headers); w > 0 && h > 0 {
				hits[i].w, hits[i].h, hits[i].ok = w, h, true
				return
			}
			switch err := player.Check(pctx, pb.URL, pb.Headers); {
			case err == nil:
				hits[i].ok = true
			case errors.Is(err, player.ErrCheckTimeout) || pctx.Err() != nil:
				hits[i].timedOut = true
			default:
				hits[i].reason = err.Error()
			}
		}(i, ln.Name, ln.Episodes[0])
	}
	wg.Wait()
	var out offerProbe
	checked, failed := 0, 0
	for _, one := range hits {
		if !one.checked {
			continue
		}
		checked++
		if one.ok && (one.h > out.h || (one.h == out.h && one.w > out.w)) {
			out.w, out.h = one.w, one.h
		}
		if !one.ok && !one.timedOut {
			failed++
			if out.reason == "" {
				out.reason = one.reason
			}
		}
	}
	switch {
	case checked == 0:
		out.unplayable, out.reason = true, "没有可播放的剧集"
	case failed == checked:
		out.unplayable = true
	default:
		out.reason = ""
	}
	return out
}
