package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoInsertAndGet(t *testing.T) {
	d := openTestDB(t)
	repo, err := d.InsertRepo("/home/user/project", "git@github.com:user/project.git", "main")
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}
	if repo.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if repo.WorkingPath != "/home/user/project" {
		t.Errorf("working path = %q, want %q", repo.WorkingPath, "/home/user/project")
	}
	if repo.UpstreamURL != "git@github.com:user/project.git" {
		t.Errorf("upstream url = %q, want %q", repo.UpstreamURL, "git@github.com:user/project.git")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("default branch = %q, want %q", repo.DefaultBranch, "main")
	}
	if repo.CreatedAt == 0 {
		t.Error("expected non-zero created_at")
	}

	got, err := d.GetRepo(repo.ID)
	if err != nil {
		t.Fatalf("get repo: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil repo")
	}
	if got.ID != repo.ID {
		t.Errorf("id = %q, want %q", got.ID, repo.ID)
	}
}

func TestInsertRepoWithID(t *testing.T) {
	d := openTestDB(t)
	repo, err := d.InsertRepoWithID("custom-id-123", "/home/user/myproject", "git@github.com:user/myproject.git", "develop")
	if err != nil {
		t.Fatalf("insert repo with id: %v", err)
	}
	if repo.ID != "custom-id-123" {
		t.Errorf("id = %q, want %q", repo.ID, "custom-id-123")
	}
	if repo.WorkingPath != "/home/user/myproject" {
		t.Errorf("working path = %q, want %q", repo.WorkingPath, "/home/user/myproject")
	}
	if repo.UpstreamURL != "git@github.com:user/myproject.git" {
		t.Errorf("upstream url = %q, want %q", repo.UpstreamURL, "git@github.com:user/myproject.git")
	}
	if repo.DefaultBranch != "develop" {
		t.Errorf("default branch = %q, want %q", repo.DefaultBranch, "develop")
	}
	if repo.CreatedAt == 0 {
		t.Error("expected non-zero created_at")
	}

	// Verify round-trip through GetRepo.
	got, err := d.GetRepo("custom-id-123")
	if err != nil {
		t.Fatalf("get repo: %v", err)
	}
	if got == nil || got.ID != "custom-id-123" {
		t.Fatal("expected repo with custom ID")
	}
	if got.DefaultBranch != "develop" {
		t.Errorf("default branch after get = %q, want %q", got.DefaultBranch, "develop")
	}
}

func TestInsertRepoWithIDDuplicate(t *testing.T) {
	d := openTestDB(t)
	_, err := d.InsertRepoWithID("dup-id", "/path/a", "git@github.com:a/b.git", "main")
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Same ID should fail (primary key constraint).
	_, err = d.InsertRepoWithID("dup-id", "/path/b", "git@github.com:c/d.git", "main")
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestRepoGetByPath(t *testing.T) {
	d := openTestDB(t)
	repo, _ := d.InsertRepo("/home/user/project", "git@github.com:user/project.git", "main")

	got, err := d.GetRepoByPath("/home/user/project")
	if err != nil {
		t.Fatalf("get repo by path: %v", err)
	}
	if got == nil || got.ID != repo.ID {
		t.Fatalf("expected repo with ID %q", repo.ID)
	}

	got, err = d.GetRepoByPath("/nonexistent")
	if err != nil {
		t.Fatalf("get repo by path (not found): %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent path")
	}
}

func TestRepoGetNotFound(t *testing.T) {
	d := openTestDB(t)
	got, err := d.GetRepo("nonexistent")
	if err != nil {
		t.Fatalf("get repo: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent repo")
	}
}

func TestRepoUniqueWorkingPath(t *testing.T) {
	d := openTestDB(t)
	_, err := d.InsertRepo("/home/user/project", "git@github.com:a/b.git", "main")
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = d.InsertRepo("/home/user/project", "git@github.com:c/d.git", "main")
	if err == nil {
		t.Fatal("expected error for duplicate working_path")
	}
}

func TestRepoDelete(t *testing.T) {
	d := openTestDB(t)
	repo, _ := d.InsertRepo("/home/user/project", "git@github.com:user/project.git", "main")

	if err := d.DeleteRepo(repo.ID); err != nil {
		t.Fatalf("delete repo: %v", err)
	}
	got, _ := d.GetRepo(repo.ID)
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

// TestRepoForkURLRoundTrip ensures the fork_url column survives a write via
// UpdateRepoMetadata and a read via GetRepo/GetRepoByPath. This is the storage
// backbone of fork-based contributions (push to fork, PR to parent).
func TestRepoForkURLRoundTrip(t *testing.T) {
	d := openTestDB(t)
	repo, err := d.InsertRepo("/home/user/project", "git@github.com:user/project.git", "main")
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}
	if repo.ForkURL != "" {
		t.Fatalf("fresh repo ForkURL = %q, want empty", repo.ForkURL)
	}

	updated, err := d.UpdateRepoMetadata(repo.ID, "git@github.com:kunchenguid/firstmate.git", "git@github.com:e-jung/firstmate.git", "main")
	if err != nil {
		t.Fatalf("update repo metadata: %v", err)
	}
	if updated.UpstreamURL != "git@github.com:kunchenguid/firstmate.git" {
		t.Errorf("UpstreamURL = %q, want parent", updated.UpstreamURL)
	}
	if updated.ForkURL != "git@github.com:e-jung/firstmate.git" {
		t.Errorf("ForkURL = %q, want fork", updated.ForkURL)
	}

	got, err := d.GetRepo(repo.ID)
	if err != nil {
		t.Fatalf("get repo: %v", err)
	}
	if got.ForkURL != "git@github.com:e-jung/firstmate.git" {
		t.Errorf("GetRepo ForkURL = %q, want fork", got.ForkURL)
	}

	gotByPath, err := d.GetRepoByPath("/home/user/project")
	if err != nil {
		t.Fatalf("get repo by path: %v", err)
	}
	if gotByPath.ForkURL != "git@github.com:e-jung/firstmate.git" {
		t.Errorf("GetRepoByPath ForkURL = %q, want fork", gotByPath.ForkURL)
	}

	// Clearing the fork URL must persist (NULL), restoring non-fork behavior.
	cleared, err := d.UpdateRepoMetadata(repo.ID, "git@github.com:kunchenguid/firstmate.git", "", "main")
	if err != nil {
		t.Fatalf("clear fork url: %v", err)
	}
	if cleared.ForkURL != "" {
		t.Errorf("after clearing, ForkURL = %q, want empty", cleared.ForkURL)
	}
}

// TestRepoPushURL asserts the single decision point used by the push step:
// fork when set, upstream otherwise.
func TestRepoPushURL(t *testing.T) {
	if got := (&Repo{UpstreamURL: "parent", ForkURL: ""}).PushURL(); got != "parent" {
		t.Errorf("PushURL() non-fork = %q, want parent", got)
	}
	if got := (&Repo{UpstreamURL: "parent", ForkURL: "fork"}).PushURL(); got != "fork" {
		t.Errorf("PushURL() fork = %q, want fork", got)
	}
	if got := (&Repo{}).PushURL(); got != "" {
		t.Errorf("PushURL() empty = %q, want empty", got)
	}
}

// TestOpenMigratesReposForkURLColumn verifies a legacy database (created
// before the fork_url column existed) gets the column added on Open, and that
// legacy rows read back as fork_url == "" (no fork).
func TestOpenMigratesReposForkURLColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")

	legacyDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := legacyDB.Exec(`
		CREATE TABLE repos (
			id             TEXT PRIMARY KEY,
			working_path   TEXT NOT NULL UNIQUE,
			upstream_url   TEXT NOT NULL,
			default_branch TEXT NOT NULL DEFAULT 'main',
			created_at     INTEGER NOT NULL
		);
		INSERT INTO repos (id, working_path, upstream_url, default_branch, created_at)
			VALUES ('legacy-1', '/legacy/path', 'git@github.com:legacy/repo.git', 'main', 0);
	`); err != nil {
		legacyDB.Close()
		t.Fatalf("seed legacy repos table: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	rows, err := d.sql.Query(`PRAGMA table_info(repos)`)
	if err != nil {
		t.Fatalf("pragma table_info(repos): %v", err)
	}
	var columns []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		columns = append(columns, name)
	}
	rows.Close()
	if !contains(columns, "fork_url") {
		t.Fatalf("expected migrated fork_url column, got %v", columns)
	}

	// Legacy row must read back without error and report no fork.
	got, err := d.GetRepo("legacy-1")
	if err != nil {
		t.Fatalf("get legacy repo after migration: %v", err)
	}
	if got == nil || got.ForkURL != "" {
		t.Fatalf("legacy repo ForkURL = %q, want empty", got.ForkURL)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
