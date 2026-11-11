package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/harrylyx/QuickBo/internal/config"
	"github.com/harrylyx/QuickBo/internal/model"
	"github.com/harrylyx/QuickBo/internal/player"
	"github.com/harrylyx/QuickBo/internal/source"
)

// measureOffer reads the real picture size of one source. Each play line
// contributes its first episode, and the reads run together. The largest
// picture wins.
func (a *App) measureOffer(ctx context.Context, src source.Source, d model.Detail, parses []config.ParseItem) (int, int) {
	lines := d.Lines
	if len(lines) > 8 {
		lines = lines[:8]
	}
	type hit struct{ w, h int }
	hits := make([]hit, len(lines))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, ln := range lines {
		if len(ln.Episodes) == 0 {
			continue
		}
		wg.Add(1)
		go func(i int, name string, ep model.Episode) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			pb, err := src.Resolve(pctx, name, ep)
			if err != nil || strings.TrimSpace(pb.URL) == "" {
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
			hits[i].w, hits[i].h = player.Measure(pctx, pb.URL, pb.Headers)
		}(i, ln.Name, ln.Episodes[0])
	}
	wg.Wait()
	var bestW, bestH int
	for _, one := range hits {
		if one.h > bestH || (one.h == bestH && one.w > bestW) {
			bestW, bestH = one.w, one.h
		}
	}
	return bestW, bestH
}
