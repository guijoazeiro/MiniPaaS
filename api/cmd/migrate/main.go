package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	for _, p := range []string{".env", "../.env"} {
		_ = godotenv.Load(p)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	source := flag.String("source", "file://sql/migrations", "migrations source URL")
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "up"
	}

	m, err := migrate.New(*source, pgxURL(dbURL))
	if err != nil {
		log.Error("migrate.New", "err", err)
		os.Exit(1)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Warn("migrate.Close", "src_err", srcErr, "db_err", dbErr)
		}
	}()

	if err := run(m, cmd, flag.Arg(1)); err != nil {
		log.Error("migrate", "cmd", cmd, "err", err)
		os.Exit(1)
	}
	log.Info("migrate ok", "cmd", cmd)
}

func run(m *migrate.Migrate, cmd, arg string) error {
	switch cmd {
	case "up":
		return skipNoChange(m.Up())
	case "down":
		n, err := parseSteps(arg, 1)
		if err != nil {
			return err
		}
		return skipNoChange(m.Steps(-n))
	case "force":
		v, err := strconv.Atoi(arg)
		if err != nil {
			return fmt.Errorf("force requires a version integer: %w", err)
		}
		return m.Force(v)
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			return err
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q (use: up | down [N] | force <v> | version)", cmd)
	}
}

func parseSteps(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid step count %q", s)
	}
	return n, nil
}

func skipNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func pgxURL(u string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if len(u) >= len(p) && u[:len(p)] == p {
			return "pgx5://" + u[len(p):]
		}
	}
	return u
}
