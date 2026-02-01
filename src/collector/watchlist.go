package collector

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type ChannelRole string

const (
	RoleSelf  ChannelRole = "self"
	RoleWatch ChannelRole = "watch"
)

type ChannelWatchlist struct {
	// logins are always stored normalized (lowercase).
	selfLogin string
	watch     []string

	roleByLogin map[string]ChannelRole
}

func NewChannelWatchlist(selfLogin string, watchLogins []string) (ChannelWatchlist, error) {
	wl := ChannelWatchlist{
		selfLogin:   normalizeLogin(selfLogin),
		watch:       make([]string, 0, len(watchLogins)),
		roleByLogin: map[string]ChannelRole{},
	}

	if wl.selfLogin != "" {
		wl.roleByLogin[wl.selfLogin] = RoleSelf
	}

	seen := map[string]struct{}{}
	if wl.selfLogin != "" {
		seen[wl.selfLogin] = struct{}{}
	}

	for _, raw := range watchLogins {
		login := normalizeLogin(raw)
		if login == "" {
			continue
		}
		if _, ok := seen[login]; ok {
			continue
		}
		seen[login] = struct{}{}
		wl.watch = append(wl.watch, login)
		wl.roleByLogin[login] = RoleWatch
	}

	if len(wl.watch) > 100 {
		return wl, fmt.Errorf("watchlist role=watch exceeds 100 channels (%d)", len(wl.watch))
	}

	sort.Strings(wl.watch)
	return wl, nil
}

func (w ChannelWatchlist) SelfLogin() string { return w.selfLogin }

func (w ChannelWatchlist) WatchLogins() []string {
	out := make([]string, 0, len(w.watch))
	out = append(out, w.watch...)
	return out
}

func (w ChannelWatchlist) AllLogins() []string {
	out := make([]string, 0, 1+len(w.watch))
	if w.selfLogin != "" {
		out = append(out, w.selfLogin)
	}
	out = append(out, w.watch...)
	return out
}

func (w ChannelWatchlist) RoleForLogin(login string) ChannelRole {
	login = normalizeLogin(login)
	if login == "" {
		return ""
	}
	if role, ok := w.roleByLogin[login]; ok {
		return role
	}
	return ""
}

func (w ChannelWatchlist) RoleLabelForLogin(login string) string {
	return string(w.RoleForLogin(login))
}

func (w ChannelWatchlist) CountByRole(role ChannelRole) int {
	switch role {
	case RoleSelf:
		if w.selfLogin == "" {
			return 0
		}
		return 1
	case RoleWatch:
		return len(w.watch)
	default:
		return 0
	}
}

func (w ChannelWatchlist) ValidateHasSelf() error {
	if w.selfLogin == "" {
		return errors.New("self channel is not configured")
	}
	return nil
}

func normalizeLogin(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}
