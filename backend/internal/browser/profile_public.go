package browser

// ProfilePublicView is the allowlisted representation for external profile
// management APIs. It intentionally excludes raw proxy connection strings,
// subscription URLs, local storage paths, launch arguments and diagnostic text.
type ProfilePublicView struct {
	ProfileId          string              `json:"profileId"`
	ProfileName        string              `json:"profileName"`
	CoreId             string              `json:"coreId"`
	RestoreLastSession string              `json:"restoreLastSession"`
	ProxyId            string              `json:"proxyId"`
	ProxyBindName      string              `json:"proxyBindName,omitempty"`
	MemoryLimitMB      int                 `json:"memoryLimitMb"`
	Tags               []string            `json:"tags"`
	Keywords           []string            `json:"keywords"`
	GroupId            string              `json:"groupId"`
	LaunchCode         string              `json:"launchCode"`
	RuntimeState       ProfileRuntimeState `json:"runtimeState"`
	Running            bool                `json:"running"`
	DebugReady         bool                `json:"debugReady"`
	CreatedAt          string              `json:"createdAt"`
	UpdatedAt          string              `json:"updatedAt"`
	DeletedAt          string              `json:"deletedAt"`
	LastStartAt        string              `json:"lastStartAt"`
	LastStopAt         string              `json:"lastStopAt"`
}

func NewProfilePublicView(p *Profile) *ProfilePublicView {
	if p == nil {
		return nil
	}
	return &ProfilePublicView{
		ProfileId: p.ProfileId, ProfileName: p.ProfileName,
		CoreId: p.CoreId, RestoreLastSession: p.RestoreLastSession,
		ProxyId: p.ProxyId, ProxyBindName: p.ProxyBindName,
		MemoryLimitMB: p.MemoryLimitMB, Tags: append([]string{}, p.Tags...),
		Keywords: append([]string{}, p.Keywords...), GroupId: p.GroupId,
		LaunchCode: p.LaunchCode, RuntimeState: NormalizeProfileRuntimeState(p),
		Running: p.Running, DebugReady: p.DebugReady,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		DeletedAt: p.DeletedAt, LastStartAt: p.LastStartAt,
		LastStopAt: p.LastStopAt,
	}
}

func NewProfilePublicViews(profiles []Profile) []ProfilePublicView {
	out := make([]ProfilePublicView, 0, len(profiles))
	for i := range profiles {
		out = append(out, *NewProfilePublicView(&profiles[i]))
	}
	return out
}
