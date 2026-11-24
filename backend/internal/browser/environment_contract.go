package browser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"github.com/google/uuid"
)

const EnvironmentContractSchemaVersion = 2

// EnvironmentIdentity decouples one logical browser environment from the
// storage row/profile ID used by the current database.
type EnvironmentIdentity struct {
	ProfileID     string `json:"profileId"`
	EnvironmentID string `json:"environmentId"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type EnvironmentIdentityDAO interface {
	Ensure(profileID string) (EnvironmentIdentity, error)
	Get(profileID string) (EnvironmentIdentity, error)
}

type SQLiteEnvironmentIdentityDAO struct {
	db *sql.DB
}

func NewSQLiteEnvironmentIdentityDAO(db *sql.DB) *SQLiteEnvironmentIdentityDAO {
	return &SQLiteEnvironmentIdentityDAO{db: db}
}

func (d *SQLiteEnvironmentIdentityDAO) Ensure(profileID string) (EnvironmentIdentity, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return EnvironmentIdentity{}, fmt.Errorf("profile id is required")
	}
	if d == nil || d.db == nil {
		return EnvironmentIdentity{}, fmt.Errorf("environment identity database is unavailable")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	environmentID := uuid.NewString()
	if _, err := d.db.Exec(`
		INSERT INTO browser_profile_environment_identity
			(profile_id, environment_id, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(profile_id) DO NOTHING
	`, profileID, environmentID, now, now); err != nil {
		return EnvironmentIdentity{}, fmt.Errorf("ensure environment identity: %w", err)
	}
	return d.Get(profileID)
}

func (d *SQLiteEnvironmentIdentityDAO) Get(profileID string) (EnvironmentIdentity, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return EnvironmentIdentity{}, fmt.Errorf("profile id is required")
	}
	if d == nil || d.db == nil {
		return EnvironmentIdentity{}, fmt.Errorf("environment identity database is unavailable")
	}
	var out EnvironmentIdentity
	err := d.db.QueryRow(`
		SELECT profile_id, environment_id, created_at, updated_at
		FROM browser_profile_environment_identity
		WHERE profile_id = ?
	`, profileID).Scan(&out.ProfileID, &out.EnvironmentID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return EnvironmentIdentity{}, err
	}
	return out, nil
}

type EnvironmentContractV2 struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Environment   EnvironmentContractIdentity `json:"environment"`
	Persona       EnvironmentPersona          `json:"persona"`
	KernelBinding EnvironmentKernelBinding    `json:"kernelBinding"`
	NetworkBinding EnvironmentNetworkBinding  `json:"networkBinding"`
	Storage       EnvironmentStorageBinding   `json:"storage"`
	Runtime       EnvironmentRuntimeState     `json:"runtime"`
	LegacyExtras  EnvironmentLegacyExtras     `json:"legacyExtras"`
}

type EnvironmentContractIdentity struct {
	EnvironmentID string `json:"environmentId"`
	ProfileID     string `json:"profileId"`
	Revision      string `json:"revision"`
	Source        string `json:"source"`
}

type EnvironmentPersona struct {
	Name     string   `json:"name"`
	Tags     []string `json:"tags"`
	Keywords []string `json:"keywords"`
}

type EnvironmentKernelBinding struct {
	ConfiguredCoreID string `json:"configuredCoreId,omitempty"`
	EffectiveCoreID  string `json:"effectiveCoreId,omitempty"`
	EffectiveCoreName string `json:"effectiveCoreName,omitempty"`
	Status           string `json:"status"`
}

type EnvironmentNetworkBinding struct {
	Mode          string `json:"mode"`
	ProxyID       string `json:"proxyId,omitempty"`
	ProxyBindName string `json:"proxyBindName,omitempty"`
	ConnectorType string `json:"connectorType"`
}

type EnvironmentStorageBinding struct {
	ManagedUserDataDir bool   `json:"managedUserDataDir"`
	RestoreLastSession string `json:"restoreLastSession"`
	MemoryLimitMB      int    `json:"memoryLimitMb"`
}

type EnvironmentRuntimeState struct {
	State      ProfileRuntimeState `json:"state"`
	Running    bool                `json:"running"`
	DebugReady bool                `json:"debugReady"`
}

type EnvironmentLegacyExtras struct {
	HasFingerprintArgs bool `json:"hasFingerprintArgs"`
	FingerprintArgCount int `json:"fingerprintArgCount"`
	HasLaunchArgs      bool `json:"hasLaunchArgs"`
	LaunchArgCount     int  `json:"launchArgCount"`
}

func (m *Manager) EnvironmentContract(profileID string) (*EnvironmentContractV2, error) {
	if m == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	m.InitData()
	m.Mutex.Lock()
	profile, ok := m.Profiles[strings.TrimSpace(profileID)]
	if !ok || profile == nil {
		m.Mutex.Unlock()
		return nil, fmt.Errorf("profile not found")
	}
	snapshot := cloneProfileValue(profile)
	m.Mutex.Unlock()

	environmentID := "legacy:" + snapshot.ProfileId
	source := "profile-v1-compat-transient"
	if m.EnvironmentDAO != nil {
		identity, err := m.EnvironmentDAO.Ensure(snapshot.ProfileId)
		if err != nil {
			return nil, err
		}
		environmentID = identity.EnvironmentID
		source = "profile-v1-compat"
	}

	effectiveCoreID := strings.TrimSpace(snapshot.CoreId)
	effectiveCoreName := ""
	kernelStatus := "configured"
	if effectiveCoreID == "" {
		if core, found := m.GetDefaultCore(); found {
			effectiveCoreID = core.CoreId
			effectiveCoreName = core.CoreName
			kernelStatus = "default"
		} else {
			kernelStatus = "missing"
		}
	} else if core, found := m.GetCore(effectiveCoreID); found {
		effectiveCoreName = core.CoreName
	} else {
		kernelStatus = "missing"
	}

	networkMode := "direct"
	if strings.TrimSpace(snapshot.ProxyId) != "" {
		networkMode = "proxy_pool"
	} else if strings.TrimSpace(snapshot.ProxyConfig) != "" {
		networkMode = "custom_proxy"
	}

	revision := strings.TrimSpace(snapshot.UpdatedAt)
	if revision == "" {
		revision = strings.TrimSpace(snapshot.CreatedAt)
	}
	connectorType := config.NormalizeBrowserConnectorType("")
	if m.Config != nil {
		connectorType = config.NormalizeBrowserConnectorType(m.Config.Browser.DefaultConnectorType)
	}
	return &EnvironmentContractV2{
		SchemaVersion: EnvironmentContractSchemaVersion,
		Environment: EnvironmentContractIdentity{
			EnvironmentID: environmentID,
			ProfileID: snapshot.ProfileId,
			Revision: revision,
			Source: source,
		},
		Persona: EnvironmentPersona{
			Name: snapshot.ProfileName,
			Tags: append([]string{}, snapshot.Tags...),
			Keywords: append([]string{}, snapshot.Keywords...),
		},
		KernelBinding: EnvironmentKernelBinding{
			ConfiguredCoreID: strings.TrimSpace(snapshot.CoreId),
			EffectiveCoreID: effectiveCoreID,
			EffectiveCoreName: effectiveCoreName,
			Status: kernelStatus,
		},
		NetworkBinding: EnvironmentNetworkBinding{
			Mode: networkMode,
			ProxyID: strings.TrimSpace(snapshot.ProxyId),
			ProxyBindName: strings.TrimSpace(snapshot.ProxyBindName),
			ConnectorType: connectorType,
		},
		Storage: EnvironmentStorageBinding{
			ManagedUserDataDir: true,
			RestoreLastSession: NormalizeRestoreLastSessionMode(snapshot.RestoreLastSession),
			MemoryLimitMB: normalizeMemoryLimitMB(snapshot.MemoryLimitMB),
		},
		Runtime: EnvironmentRuntimeState{
			State: NormalizeProfileRuntimeState(&snapshot),
			Running: snapshot.Running,
			DebugReady: snapshot.DebugReady,
		},
		LegacyExtras: EnvironmentLegacyExtras{
			HasFingerprintArgs: len(snapshot.FingerprintArgs) > 0,
			FingerprintArgCount: len(snapshot.FingerprintArgs),
			HasLaunchArgs: len(snapshot.LaunchArgs) > 0,
			LaunchArgCount: len(snapshot.LaunchArgs),
		},
	}, nil
}
