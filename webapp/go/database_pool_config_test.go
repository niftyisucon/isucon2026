package isureal

import "testing"

func TestCoreDatabaseConnectionLimit(t *testing.T) {
	core := &Config{ListenRegions: "asia:8080,europe:8081,america:8082"}
	for value, want := range map[string]int{"": 30, "30": 30, "45": 45, "60": 60} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ISUREAL_CORE_DB_MAX_OPEN", value)
			if got := databaseConnectionLimit(core); got != want {
				t.Fatalf("configured=%q got=%d want=%d", value, got, want)
			}
			for _, config := range []*Config{nil, {}, {Region: "europe"}, {ListenRegions: "asia:8080"}, {EdgeUpstream: "http://127.0.0.1:8081", ListenRegions: "europe:8080"}} {
				if got := databaseConnectionLimit(config); got != maxDBConnections {
					t.Fatalf("non-shared-core changed limit: %d", got)
				}
			}
		})
	}
	for _, value := range []string{"0", "-1", "31", "200", "unlimited", "60 "} {
		t.Run("invalid-"+value, func(t *testing.T) {
			t.Setenv("ISUREAL_CORE_DB_MAX_OPEN", value)
			defer func() {
				if recover() == nil {
					t.Fatal("invalid shared-core capacity accepted")
				}
			}()
			databaseConnectionLimit(core)
		})
	}
}
