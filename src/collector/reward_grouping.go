package collector

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	rewardGroupMu sync.RWMutex
	rewardGroups  = rewardGroupConfig{
		defaultGroup: "default",
		unknownGroup: "other",
		maxGroups:    20,
		byID:         map[string]string{},
		byTitle:      map[string]string{},
	}
)

type rewardGroupConfig struct {
	defaultGroup string
	unknownGroup string
	maxGroups    int

	byID    map[string]string
	byTitle map[string]string
}

func SetRewardGrouping(defaultGroup string, unknownGroup string, maxGroups int, byID map[string]string, byTitle map[string]string) error {
	if strings.TrimSpace(defaultGroup) == "" {
		defaultGroup = "default"
	}
	if strings.TrimSpace(unknownGroup) == "" {
		unknownGroup = "other"
	}
	if maxGroups <= 0 {
		maxGroups = 20
	}

	groups := map[string]struct{}{defaultGroup: {}, unknownGroup: {}}
	for _, g := range byID {
		groups[g] = struct{}{}
	}
	for _, g := range byTitle {
		groups[g] = struct{}{}
	}
	if len(groups) > maxGroups {
		keys := make([]string, 0, len(groups))
		for k := range groups {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Errorf("reward_group cardinality too high: %d groups (max %d): %v", len(groups), maxGroups, keys)
	}

	normID := map[string]string{}
	for k, v := range byID {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		normID[k] = v
	}

	normTitle := map[string]string{}
	for k, v := range byTitle {
		k = normalizeTitle(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		normTitle[k] = v
	}

	rewardGroupMu.Lock()
	defer rewardGroupMu.Unlock()
	rewardGroups = rewardGroupConfig{
		defaultGroup: defaultGroup,
		unknownGroup: unknownGroup,
		maxGroups:    maxGroups,
		byID:         normID,
		byTitle:      normTitle,
	}
	return nil
}

func RewardGroupFor(rewardID string, rewardTitle string) string {
	rewardGroupMu.RLock()
	cfg := rewardGroups
	rewardGroupMu.RUnlock()

	rewardID = strings.TrimSpace(rewardID)
	if rewardID != "" {
		if g, ok := cfg.byID[rewardID]; ok {
			return g
		}
	}

	title := normalizeTitle(rewardTitle)
	if title != "" {
		if g, ok := cfg.byTitle[title]; ok {
			return g
		}
	}

	if rewardID == "" && title == "" {
		return cfg.defaultGroup
	}
	return cfg.unknownGroup
}

func normalizeTitle(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ToLower(v)
	return v
}
