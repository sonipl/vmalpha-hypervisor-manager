package config

import (
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestDatabaseDSNRoundTrip(t *testing.T) {
	for _, input := range []DatabaseConfig{
		{Host: "127.0.0.1", Port: 5432, User: "operator", Password: "space ' quote = & /", DBName: "platform", SSLMode: "require"},
		{Host: "::1", Port: 5432, User: "operator", Password: "test", DBName: "platform", SSLMode: "require"},
		{Host: "/var/run/postgresql", Port: 5432, User: "vmalpha", Password: "", DBName: "vmalpha", SSLMode: "disable"},
	} {
		got, err := pgx.ParseConfig(input.DSN())
		if err != nil {
			t.Fatal(err)
		}
		if got.Host != input.Host || int(got.Port) != input.Port || got.User != input.User || got.Password != input.Password || got.Database != input.DBName {
			t.Fatal("Database connection fields did not round trip")
		}
	}
}
