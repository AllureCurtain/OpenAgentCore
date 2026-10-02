package processconfig

import (
	"strings"
	"testing"
)

func TestCheckRejectsInvalidValuesWithoutEchoingThem(t *testing.T) {
	secret := "https://user:synthetic-secret@core.example"
	t.Setenv("OAC_PUBLIC_URL", secret)
	err := Check()
	if err == nil || strings.Contains(err.Error(), "synthetic-secret") || !strings.Contains(err.Error(), "OAC_PUBLIC_URL") {
		t.Fatal(err)
	}
	t.Setenv("OAC_PUBLIC_URL", "")
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "0")
	if err := Check(); err == nil || !strings.Contains(err.Error(), "OAC_EXECUTION_CONCURRENCY") || strings.Contains(err.Error(), "synthetic") {
		t.Fatal(err)
	}
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "4")
	t.Setenv("OAC_LOG_LEVEL", "verbose")
	if err := Check(); err == nil || !strings.Contains(err.Error(), "OAC_LOG_LEVEL") {
		t.Fatal(err)
	}
}

func TestSettingsReportEffectiveValuesAndHideHistory(t *testing.T) {
	t.Setenv("OAC_PUBLIC_URL", "https://core.example")
	t.Setenv("OAC_EXECUTION_CONCURRENCY", "8")
	t.Setenv("OAC_HISTORY_SETTINGS_FILE", "/tmp/history.json")
	settings, err := Settings()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]any{}
	for _, setting := range settings {
		if setting.Sensitive && (setting.Value != nil || setting.Configured == nil) {
			t.Fatalf("sensitive setting %s leaked a value", setting.Key)
		}
		found[setting.Key] = setting.Value
		if setting.Key == "core.runtime_history" && (setting.Configured == nil || !*setting.Configured) {
			t.Fatal("history file was not reported as configured")
		}
	}
	if found["public_url"] != "https://core.example" || found["core.execution_concurrency"] != 8 {
		t.Fatal(found)
	}
}
