// Command migration_integrity runs read-only production data checks.
//
// It deliberately uses a READ ONLY transaction. A non-zero result from an
// integrity query fails the command; table counts are emitted as evidence but
// are not themselves pass/fail assertions.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type checkDefinition struct {
	Name  string
	Query string
}

type checkResult struct {
	Name   string `json:"name"`
	Value  int64  `json:"value"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type tableCount struct {
	Table string `json:"table"`
	Count int64  `json:"count"`
}

type report struct {
	GeneratedAt     time.Time     `json:"generated_at"`
	Database        string        `json:"database"`
	MigrationChecks []checkResult `json:"integrity_checks"`
	TableCounts     []tableCount  `json:"table_counts"`
	Passed          bool          `json:"passed"`
}

var integrityChecks = []checkDefinition{
	{
		Name: "orphaned enrollments",
		Query: `
SELECT COUNT(*) FROM enrollments e
LEFT JOIN students s ON s.id = e.student_id
LEFT JOIN classes c ON c.id = e.class_id
LEFT JOIN terms t ON t.id = e.term_id
WHERE s.id IS NULL OR c.id IS NULL OR t.id IS NULL`,
	},
	{
		Name: "orphaned schedules",
		Query: `
SELECT COUNT(*) FROM schedules s
LEFT JOIN terms t ON t.id = s.term_id
LEFT JOIN classes c ON c.id = s.class_id
LEFT JOIN subjects sub ON sub.id = s.subject_id
WHERE t.id IS NULL OR c.id IS NULL OR sub.id IS NULL`,
	},
	{
		Name: "orphaned class subjects",
		Query: `
SELECT COUNT(*) FROM class_subjects cs
LEFT JOIN classes c ON c.id = cs.class_id
LEFT JOIN subjects s ON s.id = cs.subject_id
WHERE c.id IS NULL OR s.id IS NULL`,
	},
	{
		Name: "orphaned grades",
		Query: `
SELECT COUNT(*) FROM grades g
LEFT JOIN enrollments e ON e.id = g.enrollment_id
LEFT JOIN subjects s ON s.id = g.subject_id
LEFT JOIN grade_components gc ON gc.id = g.component_id
WHERE e.id IS NULL OR s.id IS NULL OR gc.id IS NULL`,
	},
	{
		Name: "orphaned final grades",
		Query: `
SELECT COUNT(*) FROM grade_finals gf
LEFT JOIN enrollments e ON e.id = gf.enrollment_id
LEFT JOIN subjects s ON s.id = gf.subject_id
WHERE e.id IS NULL OR s.id IS NULL`,
	},
	{
		Name: "orphaned daily attendance",
		Query: `
SELECT COUNT(*) FROM daily_attendance da
LEFT JOIN enrollments e ON e.id = da.enrollment_id
WHERE e.id IS NULL`,
	},
	{
		Name: "orphaned subject attendance",
		Query: `
SELECT COUNT(*) FROM subject_attendance sa
LEFT JOIN enrollments e ON e.id = sa.enrollment_id
LEFT JOIN schedules s ON s.id = sa.schedule_id
WHERE e.id IS NULL OR s.id IS NULL`,
	},
	{
		Name: "orphaned behavior notes",
		Query: `
SELECT COUNT(*) FROM behavior_notes bn
LEFT JOIN students s ON s.id = bn.student_id
WHERE s.id IS NULL`,
	},
	{
		Name: "orphaned teacher assignments",
		Query: `
SELECT COUNT(*) FROM teacher_assignments ta
LEFT JOIN teachers t ON t.id = ta.teacher_id
LEFT JOIN classes c ON c.id = ta.class_id
LEFT JOIN subjects s ON s.id = ta.subject_id
LEFT JOIN terms term ON term.id = ta.term_id
WHERE t.id IS NULL OR c.id IS NULL OR s.id IS NULL OR term.id IS NULL`,
	},
	{
		Name: "orphaned semester schedules",
		Query: `
SELECT COUNT(*) FROM semester_schedules ss
LEFT JOIN terms t ON t.id = ss.term_id
LEFT JOIN classes c ON c.id = ss.class_id
WHERE t.id IS NULL OR c.id IS NULL`,
	},
	{
		Name: "orphaned semester schedule slots",
		Query: `
SELECT COUNT(*) FROM semester_schedule_slots slot
LEFT JOIN semester_schedules ss ON ss.id = slot.semester_schedule_id
LEFT JOIN subjects s ON s.id = slot.subject_id
LEFT JOIN teachers t ON t.id = slot.teacher_id
WHERE ss.id IS NULL OR s.id IS NULL OR t.id IS NULL`,
	},
	{
		Name: "orphaned password reset tokens",
		Query: `
SELECT COUNT(*) FROM password_reset_tokens token
LEFT JOIN users u ON u.id = token.user_id
WHERE u.id IS NULL`,
	},
	{
		Name: "grade calculation discrepancies",
		Query: `
SELECT COUNT(*) FROM (
  SELECT gf.id
  FROM grade_finals gf
  JOIN enrollments e ON e.id = gf.enrollment_id
  JOIN grade_configs config
    ON config.class_id = e.class_id
   AND config.subject_id = gf.subject_id
   AND config.term_id = e.term_id
  JOIN grade_config_components component ON component.grade_config_id = config.id
  JOIN grades g
    ON g.enrollment_id = gf.enrollment_id
   AND g.subject_id = gf.subject_id
   AND g.component_id = component.component_id
   AND g.deleted_at IS NULL
  GROUP BY gf.id, gf.final_grade
  HAVING ABS(gf.final_grade - SUM(g.grade_value * component.weight)) > 0.01
) discrepancies`,
	},
}

var countedTables = []string{
	"users", "students", "classes", "subjects", "terms", "teachers",
	"enrollments", "grades", "grade_finals", "daily_attendance",
	"subject_attendance", "announcements", "behavior_notes", "calendar_events",
	"refresh_tokens", "audit_logs", "report_jobs", "mutations", "archives",
	"semester_schedules", "semester_schedule_slots", "configurations",
	"parent_students", "portal_preferences", "device_tokens", "notification_queue",
	"archive_documents", "password_reset_tokens",
}

func dsnFromEnvironment() (string, error) {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv("DB_URL")); value != "" {
		return value, nil
	}
	host := getenvDefault("DB_HOST", "localhost")
	port := getenvDefault("DB_PORT", "5432")
	user := getenvDefault("DB_USER", "postgres")
	password := os.Getenv("DB_PASSWORD")
	database := getenvDefault("DB_NAME", "admin_panel_sma")
	sslMode := getenvDefault("DB_SSL_MODE", "require")
	if password == "" && os.Getenv("ENV") == "production" {
		return "", errors.New("DATABASE_URL or DB_PASSWORD is required in production")
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, database, sslMode), nil
}

func getenvDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func runChecks(ctx context.Context, db *sqlx.DB) (report, error) {
	if db == nil {
		return report{}, errors.New("database handle is nil")
	}
	tx, err := db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report{}, fmt.Errorf("begin read-only transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		return report{}, fmt.Errorf("enforce read-only transaction: %w", err)
	}

	result := report{GeneratedAt: time.Now().UTC(), Passed: true}
	for _, definition := range integrityChecks {
		var count int64
		if err := tx.GetContext(ctx, &count, definition.Query); err != nil {
			return report{}, fmt.Errorf("%s: %w", definition.Name, err)
		}
		result.MigrationChecks = append(result.MigrationChecks, checkResult{
			Name:   definition.Name,
			Value:  count,
			Passed: count == 0,
			Detail: fmt.Sprintf("%d violating rows", count),
		})
		if count != 0 {
			result.Passed = false
		}
	}

	for _, table := range countedTables {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		if err := tx.GetContext(ctx, &count, query); err != nil {
			return report{}, fmt.Errorf("count %s: %w", table, err)
		}
		result.TableCounts = append(result.TableCounts, tableCount{Table: table, Count: count})
	}
	if err := tx.Commit(); err != nil {
		return report{}, fmt.Errorf("commit read-only transaction: %w", err)
	}
	return result, nil
}

func printReport(result report, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	fmt.Printf("Migration integrity report (%s)\n", result.GeneratedAt.Format(time.RFC3339))
	for _, check := range result.MigrationChecks {
		status := "PASS"
		if !check.Passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-38s %s\n", status, check.Name, check.Detail)
	}
	fmt.Println("Table counts:")
	for _, count := range result.TableCounts {
		fmt.Printf("  %-28s %d\n", count.Table, count.Count)
	}
	return nil
}

func main() {
	var (
		dsn     string
		timeout time.Duration
		asJSON  bool
	)
	flag.StringVar(&dsn, "dsn", "", "PostgreSQL DSN (defaults to DATABASE_URL, DB_URL, or DB_* variables)")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "maximum duration for the read-only validation")
	flag.BoolVar(&asJSON, "json", false, "emit machine-readable JSON evidence")
	flag.Parse()

	if dsn == "" {
		var err error
		dsn, err = dsnFromEnvironment()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(2)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping database: %v\n", err)
		os.Exit(2)
	}

	result, err := runChecks(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration integrity failed: %v\n", err)
		os.Exit(2)
	}
	if err := printReport(result, asJSON); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(2)
	}
	if !result.Passed {
		os.Exit(1)
	}
}

func init() {
	// Keep deterministic output for evidence consumers that sort checks by name.
	sort.SliceStable(integrityChecks, func(i, j int) bool {
		return integrityChecks[i].Name < integrityChecks[j].Name
	})
}
