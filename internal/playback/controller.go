// Package playback resolves media addresses and drives the mpv player. It is
// the seam between the UI (internal/app) and the player kernel
// (internal/player and internal/player/mpvproc).
package playback

import (
	"context"

	"github.com/harrylyx/QuickBo/internal/player"
	"github.com/harrylyx/QuickBo/internal/player/mpvplugin"
	"github.com/harrylyx/QuickBo/internal/player/mpvproc"
)

// Controller owns mpv discovery and player construction.
type Controller struct {
	mpvPath string
}

// New returns a controller. mpv is located and preflighted on first Play.
func New() *Controller { return &Controller{} }

// Play locates mpv, preflights it and starts playback of raw. The returned
// player is driven by the UI; the video renders in mpv's own window.
func (c *Controller) Play(ctx context.Context, raw string, headers map[string]string) (player.Player, error) {
	exe, err := c.mpv()
	if err != nil {
		return nil, err
	}
	p, err := mpvproc.New(exe)
	if err != nil {
		return nil, err
	}
	if err := p.Load(ctx, raw, headers); err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

func (c *Controller) mpv() (string, error) {
	if c.mpvPath != "" {
		return c.mpvPath, nil
	}
	path, err := mpvplugin.Find()
	if err != nil {
		return "", err
	}
	if _, err := mpvplugin.Check(path); err != nil {
		return "", err
	}
	c.mpvPath = path
	return path, nil
}
