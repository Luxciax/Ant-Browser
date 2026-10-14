package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProfileCreationRejectsOverlappingDirectories(t *testing.T) {
	for _, dirs := range [][2]string{{"shared", "shared"}, {"shared", "shared/child"}, {"shared/child", "shared"}} {
		t.Run(dirs[0]+"/"+dirs[1], func(t *testing.T) {
			m := NewManager(config.DefaultConfig(), t.TempDir())
			if _, err := m.Create(ProfileInput{ProfileName: "owner", UserDataDir: dirs[0]}); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Create(ProfileInput{ProfileName: "duplicate", UserDataDir: dirs[1]}); err == nil {
				t.Fatal("accepted overlapping directory")
			}
		})
	}
}

func TestProfileDeletionPreservesOtherProfilesSharedData(t *testing.T) {
	for _, expired := range []bool{false, true} {
		for _, otherDir := range []string{"shared", "shared/child"} {
			t.Run(fmt.Sprintf("%s/expired=%v", otherDir, expired), func(t *testing.T) {
				root := t.TempDir()
				m := NewManager(config.DefaultConfig(), root)
				db, err := database.NewDB(filepath.Join(root, "audit.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				if err := db.Migrate(); err != nil {
					t.Fatal(err)
				}
				m.ProfileDAO = NewSQLiteProfileDAO(db.GetConn())
				for _, p := range []*Profile{{ProfileId: "trash", UserDataDir: "shared"}, {ProfileId: "live", UserDataDir: otherDir}} {
					if err := m.ProfileDAO.Upsert(p); err != nil {
						t.Fatal(err)
					}
				}
				if err := m.ProfileDAO.SoftDelete("trash", time.Now().Add(-96*time.Hour).Format(time.RFC3339)); err != nil {
					t.Fatal(err)
				}
				if _, err := m.Create(ProfileInput{UserDataDir: "shared"}); err == nil {
					t.Fatal("reused a directory still owned by trash")
				}
				data := filepath.Join(m.ResolveUserDataDir(&Profile{ProfileId: "live", UserDataDir: otherDir}), "Default", "Preferences")
				auditWriteFile(t, data, "keep login state")
				if expired {
					err = m.CleanupExpiredTrash()
				} else {
					err = m.PermanentlyDelete("trash")
				}
				if err == nil {
					t.Error("deletion accepted a directory owned by another profile")
				}
				if content, err := os.ReadFile(data); err != nil || string(content) != "keep login state" {
					t.Fatalf("other profile data lost: %q %v", content, err)
				}
				if _, err := m.ProfileDAO.GetById("trash"); err != nil {
					t.Fatalf("trash record removed: %v", err)
				}
			})
		}
	}
}

func TestProfileUpdateRejectsDirectoryOverlapWithoutChangingState(t *testing.T) {
	m := NewManager(config.DefaultConfig(), t.TempDir())
	if _, err := m.Create(ProfileInput{ProfileName: "owner", UserDataDir: "shared"}); err != nil {
		t.Fatal(err)
	}
	p, err := m.Create(ProfileInput{ProfileName: "editable", UserDataDir: "separate"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Update(p.ProfileId, ProfileInput{ProfileName: "changed", UserDataDir: "shared/child"}); err == nil {
		t.Fatal("accepted overlap")
	}
	if p.UserDataDir != "separate" || p.ProfileName != "editable" {
		t.Fatalf("failed edit mutated profile: %#v", p)
	}
	if _, err := m.Update(p.ProfileId, ProfileInput{ProfileName: "renamed", UserDataDir: "separate"}); err != nil {
		t.Fatalf("editing own directory: %v", err)
	}
}
