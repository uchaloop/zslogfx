package zslogfx

import (
	"reflect"
	"testing"
)

func TestConfigEnvironmentTagStructure(t *testing.T) {
	configType := reflect.TypeOf(Config{})

	caller, ok := configType.FieldByName("Caller")
	if !ok || caller.Tag.Get("envPrefix") != "CALLER_" {
		t.Fatalf("Caller envPrefix = %q, want CALLER_", caller.Tag.Get("envPrefix"))
	}
	callerEnabled, _ := reflect.TypeOf(CallerConfig{}).FieldByName("Enabled")
	if callerEnabled.Tag.Get("env") != "ENABLED" {
		t.Fatalf("Caller.Enabled env = %q, want ENABLED", callerEnabled.Tag.Get("env"))
	}
}
