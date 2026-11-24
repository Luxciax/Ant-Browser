package browser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	NetworkDNSInherit = "inherit"
	NetworkDNSProxy   = "proxy"

	NetworkWebRTCRestricted = "restricted"
	NetworkWebRTCNative     = "native"

	NetworkLocaleManual    = "manual"
	NetworkLocaleRecommend = "recommend"
)

type NetworkPolicy struct {
	ProfileID      string `json:"profileId"`
	FailClosed     bool   `json:"failClosed"`
	DNSMode        string `json:"dnsMode"`
	WebRTCMode     string `json:"webRtcMode"`
	LocaleLinkMode string `json:"localeLinkMode"`
	UpdatedAt      string `json:"updatedAt"`
}

type NetworkPolicyDAO interface {
	Get(profileID string) (NetworkPolicy, error)
	Upsert(policy NetworkPolicy) error
	Delete(profileID string) error
}

type SQLiteNetworkPolicyDAO struct {
	db *sql.DB
}

func NewSQLiteNetworkPolicyDAO(db *sql.DB) *SQLiteNetworkPolicyDAO {
	return &SQLiteNetworkPolicyDAO{db: db}
}

func DefaultNetworkPolicy(profileID string) NetworkPolicy {
	return NetworkPolicy{
		ProfileID: strings.TrimSpace(profileID),
		FailClosed: true,
		DNSMode: NetworkDNSInherit,
		WebRTCMode: NetworkWebRTCRestricted,
		LocaleLinkMode: NetworkLocaleManual,
	}
}

func NormalizeNetworkPolicy(policy NetworkPolicy) (NetworkPolicy, error) {
	policy.ProfileID = strings.TrimSpace(policy.ProfileID)
	if policy.ProfileID == "" {
		return NetworkPolicy{}, fmt.Errorf("profile id is required")
	}
	switch strings.ToLower(strings.TrimSpace(policy.DNSMode)) {
	case "", NetworkDNSInherit:
		policy.DNSMode = NetworkDNSInherit
	case NetworkDNSProxy:
		policy.DNSMode = NetworkDNSProxy
	default:
		return NetworkPolicy{}, fmt.Errorf("dnsMode must be inherit or proxy")
	}
	switch strings.ToLower(strings.TrimSpace(policy.WebRTCMode)) {
	case "", NetworkWebRTCRestricted:
		policy.WebRTCMode = NetworkWebRTCRestricted
	case NetworkWebRTCNative:
		policy.WebRTCMode = NetworkWebRTCNative
	default:
		return NetworkPolicy{}, fmt.Errorf("webRtcMode must be restricted or native")
	}
	switch strings.ToLower(strings.TrimSpace(policy.LocaleLinkMode)) {
	case "", NetworkLocaleManual:
		policy.LocaleLinkMode = NetworkLocaleManual
	case NetworkLocaleRecommend:
		policy.LocaleLinkMode = NetworkLocaleRecommend
	default:
		return NetworkPolicy{}, fmt.Errorf("localeLinkMode must be manual or recommend")
	}
	if strings.TrimSpace(policy.UpdatedAt) == "" {
		policy.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return policy, nil
}

func (d *SQLiteNetworkPolicyDAO) Get(profileID string) (NetworkPolicy, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return NetworkPolicy{}, fmt.Errorf("profile id is required")
	}
	if d == nil || d.db == nil {
		return NetworkPolicy{}, fmt.Errorf("network policy database is unavailable")
	}
	var policy NetworkPolicy
	var failClosed int
	err := d.db.QueryRow(`
		SELECT profile_id, fail_closed, dns_mode, webrtc_mode, locale_link_mode, updated_at
		FROM browser_profile_network_policy
		WHERE profile_id = ?
	`, profileID).Scan(&policy.ProfileID, &failClosed, &policy.DNSMode, &policy.WebRTCMode, &policy.LocaleLinkMode, &policy.UpdatedAt)
	if err == sql.ErrNoRows {
		return DefaultNetworkPolicy(profileID), nil
	}
	if err != nil {
		return NetworkPolicy{}, err
	}
	policy.FailClosed = failClosed != 0
	return NormalizeNetworkPolicy(policy)
}

func (d *SQLiteNetworkPolicyDAO) Upsert(policy NetworkPolicy) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("network policy database is unavailable")
	}
	normalized, err := NormalizeNetworkPolicy(policy)
	if err != nil {
		return err
	}
	failClosed := 0
	if normalized.FailClosed {
		failClosed = 1
	}
	_, err = d.db.Exec(`
		INSERT INTO browser_profile_network_policy
			(profile_id, fail_closed, dns_mode, webrtc_mode, locale_link_mode, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			fail_closed = excluded.fail_closed,
			dns_mode = excluded.dns_mode,
			webrtc_mode = excluded.webrtc_mode,
			locale_link_mode = excluded.locale_link_mode,
			updated_at = excluded.updated_at
	`, normalized.ProfileID, failClosed, normalized.DNSMode, normalized.WebRTCMode, normalized.LocaleLinkMode, normalized.UpdatedAt)
	return err
}

func (d *SQLiteNetworkPolicyDAO) Delete(profileID string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("network policy database is unavailable")
	}
	_, err := d.db.Exec("DELETE FROM browser_profile_network_policy WHERE profile_id = ?", strings.TrimSpace(profileID))
	return err
}

func (m *Manager) GetNetworkPolicy(profileID string) (NetworkPolicy, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return NetworkPolicy{}, fmt.Errorf("profile id is required")
	}
	if m == nil || m.NetworkPolicyDAO == nil {
		return DefaultNetworkPolicy(profileID), nil
	}
	return m.NetworkPolicyDAO.Get(profileID)
}

func (m *Manager) SetNetworkPolicy(policy NetworkPolicy) (NetworkPolicy, error) {
	if m == nil {
		return NetworkPolicy{}, fmt.Errorf("browser manager is unavailable")
	}
	normalized, err := NormalizeNetworkPolicy(policy)
	if err != nil {
		return NetworkPolicy{}, err
	}
	if m.NetworkPolicyDAO == nil {
		return NetworkPolicy{}, fmt.Errorf("network policy persistence is unavailable")
	}
	m.InitData()
	m.Mutex.Lock()
	profile := m.Profiles[normalized.ProfileID]
	if profile == nil {
		m.Mutex.Unlock()
		return NetworkPolicy{}, fmt.Errorf("profile not found")
	}
	if ProfileRuntimeMutationBlocked(profile) {
		state := NormalizeProfileRuntimeState(profile)
		m.Mutex.Unlock()
		return NetworkPolicy{}, fmt.Errorf("profile runtime is %s; stop it before changing network policy", state)
	}
	previousProfile := cloneProfileValue(profile)
	previousPolicy, previousPolicyErr := m.NetworkPolicyDAO.Get(normalized.ProfileID)
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := m.NetworkPolicyDAO.Upsert(normalized); err != nil {
		m.Mutex.Unlock()
		return NetworkPolicy{}, err
	}
	var persistErr error
	if m.ProfileDAO != nil {
		persistErr = m.ProfileDAO.Upsert(profile)
	} else {
		persistErr = m.SaveProfiles()
	}
	if persistErr != nil {
		*profile = previousProfile
		if previousPolicyErr == nil {
			_ = m.NetworkPolicyDAO.Upsert(previousPolicy)
		} else {
			_ = m.NetworkPolicyDAO.Delete(normalized.ProfileID)
		}
		m.Mutex.Unlock()
		return NetworkPolicy{}, persistErr
	}
	m.Mutex.Unlock()
	return m.NetworkPolicyDAO.Get(normalized.ProfileID)
}
