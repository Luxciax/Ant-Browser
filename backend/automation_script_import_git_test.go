package backend

import "testing"

func TestValidateAutomationGitRepositoryURL(t *testing.T) {
	valid := []string{
		"https://github.com/example/repo.git",
		"https://git.example.com/team/repo",
	}
	for _, raw := range valid {
		if _, err := validateAutomationGitRepositoryURL(raw); err != nil {
			t.Errorf("valid repo URL %q rejected: %v", raw, err)
		}
	}

	invalid := []string{
		"http://github.com/example/repo.git",
		"file:///C:/repo",
		"C:\\repo",
		"git@github.com:example/repo.git",
		"https://user:secret@example.com/repo.git",
		"--upload-pack=evil",
	}
	for _, raw := range invalid {
		if _, err := validateAutomationGitRepositoryURL(raw); err == nil {
			t.Errorf("invalid repo URL %q accepted", raw)
		}
	}
}

func TestValidateAutomationGitRef(t *testing.T) {
	valid := []string{"main", "release/v1.2.3", "refs/tags/v1.0.0", "abc1234"}
	for _, raw := range valid {
		if _, err := validateAutomationGitRef(raw); err != nil {
			t.Errorf("valid ref %q rejected: %v", raw, err)
		}
	}

	invalid := []string{"--help", "../main", "feature..branch", "refs/heads/x.lock", "main^{commit}", "bad ref"}
	for _, raw := range invalid {
		if _, err := validateAutomationGitRef(raw); err == nil {
			t.Errorf("invalid ref %q accepted", raw)
		}
	}
}
