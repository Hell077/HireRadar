package postgres

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplicationProfileRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated PostgreSQL test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	userID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@application-profile.example"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	store := NewProfileStore(pool)
	profile := domain.ApplicationProfile{
		UserID: userID, FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.test",
		Phone: "+1 555 0100", Country: "GB", City: "London", Address: "10 Example Street",
		Latitude: float64Pointer(51.5), Longitude: float64Pointer(-0.1), LinkedInURL: "https://linkedin.example/ada",
		ExpectedSalary:    &domain.ApplicationMoney{Amount: 140000, Currency: "USD"},
		WorkAuthorization: []domain.WorkAuthorization{{Country: "GB", Status: "authorized"}},
		CustomAnswers:     map[string]string{"preferred_name": "Ada"},
	}
	invalidResume := profile
	invalidResume.ResumeID = uuid.NewString()
	if err := store.SaveProfile(ctx, invalidResume); !errors.Is(err, domain.ErrInvalidApplicationProfile) {
		t.Fatalf("expected invalid resume selection error, got %v", err)
	}
	if err := store.SaveProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetProfile(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FirstName != profile.FirstName || got.Email != profile.Email || got.Address != profile.Address ||
		got.Latitude == nil || *got.Latitude != *profile.Latitude || got.Longitude == nil || *got.Longitude != *profile.Longitude ||
		got.ExpectedSalary == nil || *got.ExpectedSalary != *profile.ExpectedSalary ||
		!reflect.DeepEqual(got.WorkAuthorization, profile.WorkAuthorization) || !reflect.DeepEqual(got.CustomAnswers, profile.CustomAnswers) {
		t.Fatalf("application profile round trip mismatch: %+v", got)
	}
	profile.FirstName = "Augusta"
	if err := store.SaveProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetProfile(ctx, userID)
	if err != nil || got.FirstName != "Augusta" {
		t.Fatalf("application profile upsert failed: %+v, %v", got, err)
	}
}

func float64Pointer(value float64) *float64 { return &value }
