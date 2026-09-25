package postgres

import (
	"context"
	"os"
	"testing"

	profile "github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProfilePostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	userID := uuid.NewString()
	_, err = pool.Exec(ctx, "INSERT INTO users (id,email,password_hash,email_verified) VALUES ($1,$2,'integration-hash',true)", userID, "profile-"+uuid.NewString()+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id=$1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id=$1", userID)
	}()
	store := NewStore(pool)
	id := user.UserID(userID)
	profileValue := profile.Profile{UserID: id, FirstName: "Alex", Country: "KZ", City: "Almaty", ExperienceYears: 5, Seniority: "senior"}
	if err := store.Save(ctx, profileValue); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, profileValue); err != nil {
		t.Fatal(err)
	}
	if got := eventCount(t, pool, userID); got != 1 {
		t.Fatalf("profile events = %d", got)
	}
	loaded, err := store.Get(ctx, id)
	if err != nil || loaded.Country != "KZ" {
		t.Fatalf("profile = %+v, %v", loaded, err)
	}

	years := 4.0
	skills := []profile.Skill{{Name: "go lang", Years: &years, Level: "advanced"}, {Name: "postgres"}}
	if err := store.SaveSkills(ctx, id, skills); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSkills(ctx, id, []profile.Skill{{Name: "Go", Years: &years, Level: "advanced"}, {Name: "PostgreSQL"}}); err != nil {
		t.Fatal(err)
	}
	gotSkills, err := store.GetSkills(ctx, id)
	if err != nil || len(gotSkills) != 2 || gotSkills[0].Name != "Go" {
		t.Fatalf("skills = %+v, %v", gotSkills, err)
	}
	if got := eventCount(t, pool, userID); got != 2 {
		t.Fatalf("skill events = %d", got)
	}

	positions := []string{"Go Developer", "Backend Engineer"}
	if err := store.SavePositions(ctx, id, positions); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePositions(ctx, id, []string{"backend engineer", "go developer"}); err != nil {
		t.Fatal(err)
	}
	if got := eventCount(t, pool, userID); got != 3 {
		t.Fatalf("position events = %d", got)
	}

	prefs := profile.Preferences{RemotePolicies: []string{"remote"}, EmploymentTypes: []string{"full_time"}, AllowedRegions: []string{"europe"}, ExcludedCountries: []string{"KZ"}, MinimumSalary: &profile.Money{Amount: 1000, Currency: "USD"}, MinimumMatchScore: 50, MaximumJobAgeDays: 30, NotificationsEnabled: true}
	if err := store.SavePreferences(ctx, id, prefs); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePreferences(ctx, id, prefs); err != nil {
		t.Fatal(err)
	}
	loadedPrefs, err := store.GetPreferences(ctx, id)
	if err != nil || loadedPrefs.MinimumSalary == nil || loadedPrefs.MinimumSalary.Amount != 1000 {
		t.Fatalf("preferences = %+v, %v", loadedPrefs, err)
	}
	if got := eventCount(t, pool, userID); got != 4 {
		t.Fatalf("preference events = %d", got)
	}

	sources := []profile.SourcePreference{{SourceID: "greenhouse", Enabled: true}, {SourceID: "lever", Enabled: false}}
	if err := store.SaveSourcePreferences(ctx, id, sources); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourcePreferences(ctx, id, sources); err != nil {
		t.Fatal(err)
	}
	loadedSources, err := store.GetSourcePreferences(ctx, id)
	if err != nil || len(loadedSources) != 2 {
		t.Fatalf("source preferences = %+v, %v", loadedSources, err)
	}
	if got := eventCount(t, pool, userID); got != 5 {
		t.Fatalf("source preference events = %d", got)
	}
}

func eventCount(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM outbox_events WHERE event_type='profile.changed' AND aggregate_id=$1", userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
