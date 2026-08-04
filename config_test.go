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
	buffer, ok := configType.FieldByName("Buffer")
	if !ok || buffer.Tag.Get("envPrefix") != "BUFFER_" {
		t.Fatalf("Buffer envPrefix = %q, want BUFFER_", buffer.Tag.Get("envPrefix"))
	}

	callerEnabled, _ := reflect.TypeOf(CallerConfig{}).FieldByName("Enabled")
	if callerEnabled.Tag.Get("env") != "ENABLED" {
		t.Fatalf("Caller.Enabled env = %q, want ENABLED", callerEnabled.Tag.Get("env"))
	}

	bufferType := reflect.TypeOf(BufferConfig{})
	for _, name := range []string{"Enabled", "Size", "FlushInterval"} {
		field, ok := bufferType.FieldByName(name)
		if !ok || field.Tag.Get("env") == "" {
			t.Errorf("Buffer.%s has no env tag", name)
		}
	}
}
