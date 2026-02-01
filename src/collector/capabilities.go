package collector

import "sync"

var (
	capMu sync.RWMutex
	caps  = Capabilities{}
)

type Capabilities struct {
	AppTokenPresent  bool
	UserTokenPresent bool
	UserScopes       map[string]struct{}
}

// KnownUserScopes is the bounded set of OAuth scopes this exporter understands.
var KnownUserScopes = []string{
	"bits:read",
	"channel:read:subscriptions",
	"channel:read:redemptions",
	"channel:read:ads",
	"channel:read:charity",
	"channel:read:goals",
	"channel:read:hype_train",
	"channel:read:polls",
	"channel:read:predictions",
	"moderator:read:followers",
	"moderator:read:chatters",
	"moderation:read",
	"user:read:chat",
	"chat:read",
}

func SetCapabilities(appTokenPresent bool, userTokenPresent bool, userScopes []string) {
	s := map[string]struct{}{}
	for _, scope := range userScopes {
		s[scope] = struct{}{}
	}

	capMu.Lock()
	defer capMu.Unlock()
	caps = Capabilities{
		AppTokenPresent:  appTokenPresent,
		UserTokenPresent: userTokenPresent,
		UserScopes:       s,
	}
}

func GetCapabilities() Capabilities {
	capMu.RLock()
	defer capMu.RUnlock()
	out := caps
	// copy map to avoid mutation.
	if out.UserScopes != nil {
		copyMap := map[string]struct{}{}
		for k := range out.UserScopes {
			copyMap[k] = struct{}{}
		}
		out.UserScopes = copyMap
	}
	return out
}

func HasUserScope(scope string) bool {
	capMu.RLock()
	defer capMu.RUnlock()
	if !caps.UserTokenPresent {
		return false
	}
	_, ok := caps.UserScopes[scope]
	return ok
}
