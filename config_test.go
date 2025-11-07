package configloader

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

type TestConf struct {
	Foo string
}

func TestLoadNoConfig(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}
	defer loader.Close()

	loader.RegisterCallback(func(conf TestConf) (TestConf, error) {
		if conf.Foo == "" {
			conf.Foo = "default"
		}
		return conf, nil
	})

	conf := loader.Config()
	log.Printf("Config: %#v", conf)
	if conf == nil {
		t.Fatalf("expected default config, got nil")
	}
	if conf.Foo != "default" {
		t.Errorf("expected 'foo' = 'default', got %q", conf.Foo)
	}

	loader.SetConfigPath("testdata/config.yaml", true)
	conf = loader.Config()

	if conf.Foo != "foo!" {
		t.Errorf("expected 'foo' = 'foo!', got %q", conf.Foo)
	}
}

func TestLoadConfig(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}
	defer loader.Close()

	err = loader.SetConfigPath("testdata/config.yaml", true)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}

	conf := loader.Config()

	if conf.Foo != "foo!" {
		t.Errorf("expected 'foo' = 'foo!', got %q", conf.Foo)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}
	defer loader.Close()

	err = loader.SetConfigPath("testdata/noconfig.yaml", false)
	log.Printf("err: %v", err)
	if err != nil {
		t.Fatalf("expected no error loading missing but not required config, got: %v", err)
	}

	conf := loader.Config()
	if conf == nil {
		t.Fatalf("expected non-nil config")
	}
}

func TestLoadMissingConfig2(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}
	defer loader.Close()

	err = loader.SetConfigPath("testdata/noconfig.yaml", true)
	if err == nil {
		t.Fatalf("expected error loading missing config")
	}

	conf := loader.Config()
	if conf != nil {
		t.Fatalf("expected nil config, got %v", conf)
	}

	err = loader.SetConfigPath("testdata/noconfig.yaml", false)
	log.Printf("err: %v", err)
	if err != nil {
		t.Fatalf("expected no error loading missing, not-required config, got: %v", err)
	}

	conf = loader.Config()
	if conf == nil {
		t.Fatalf("expected non-nil config")
	}
}

func TestSubscribeConfig(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}
	defer loader.Close()

	err = loader.SetConfigPath("testdata/config.yaml", true)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}

	ch := loader.Subscribe()

	conf := <-ch

	if conf.Foo != "foo!" {
		t.Errorf("expected 'foo' = 'foo!', got %q", conf.Foo)
	}
}

func TestSubscribeConfigWithTempFile(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	initial := []byte("foo: foo!\n")
	if _, err := tmpfile.Write(initial); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	tmpfile.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	err = loader.SetConfigPath(tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}

	subscription := loader.Subscribe()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	count := 0
	for {
		count++
		select {
		case conf := <-subscription:
			log.Printf("new config, Foo=%q", conf.Foo)
			if count >= 2 && conf.Foo == "newfoo!" {
				return
			}
			if count == 1 && conf.Foo != "foo!" {
				t.Errorf("expected 'foo' = 'newfoo!', got %q", conf.Foo)
			}
			if count == 2 && conf.Foo != "newfoo!" {
				t.Errorf("expected 'foo' = 'newfoo!', got %q", conf.Foo)
			}
		case <-ticker.C:
			// Update config file
			updated := []byte("foo: newfoo!\n")
			if err := os.WriteFile(tmpfile.Name(), updated, 0644); err != nil {
				t.Fatalf("failed to update config: %v", err)
			}

		}
		if count > 5 {
			log.Printf("exiting after 5 iterations")
			return
		}
	}
}

func TestConfigLoaderCallback(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	initial := []byte("foo: foo!\n")
	if _, err := tmpfile.Write(initial); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	tmpfile.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	err = loader.SetConfigPath(tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	// Register a callback that rejects configs where Foo == "bad"
	loader.RegisterCallback(func(conf TestConf) (TestConf, error) {
		if conf.Foo == "bad" {
			return conf, fmt.Errorf("invalid Foo value: bad")
		}
		return conf, nil
	})

	subscription := loader.Subscribe()

	conf := <-subscription
	if conf.Foo != "foo!" {
		t.Errorf("expected 'foo' = 'foo!', got %q", conf.Foo)
	}
	log.Printf("Initial config: %v", conf)

	// Update config with a valid value
	if err := os.WriteFile(tmpfile.Name(), []byte("foo: good\n"), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	conf = <-subscription
	if conf.Foo != "good" {
		t.Errorf("expected 'foo' = 'good', got %q", conf.Foo)
	}
	log.Printf("Updated config: %v", conf)

	// Update config with an invalid value
	if err := os.WriteFile(tmpfile.Name(), []byte("foo: bad\n"), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	time.Sleep(1 * time.Second) // Allow time for the callback to process

	conf = *loader.Config()
	if conf.Foo != "good" {
		t.Errorf("expected 'foo' = 'good', got %q", conf.Foo)
	}

}

// TestNoDualDeliveryOnSetConfigPath tests that SetConfigPath doesn't cause duplicate broadcasts
func TestNoDualDeliveryOnSetConfigPath(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	initial := []byte("foo: initial\n")
	if _, err := tmpfile.Write(initial); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	tmpfile.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	// Subscribe before setting path to catch all updates
	subscription := loader.Subscribe()

	// Set config path - this should only result in ONE broadcast
	err = loader.SetConfigPath(tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("error setting config path: %v", err)
	}

	// Collect all broadcasts within a short time window
	var configs []TestConf
	timeout := time.After(500 * time.Millisecond)

	for {
		select {
		case conf := <-subscription:
			configs = append(configs, conf)
			log.Printf("Received config #%d: %+v", len(configs), conf)

		case <-timeout:
			// Ensure we got exactly one broadcast
			if len(configs) != 1 {
				t.Errorf("Expected exactly 1 config broadcast from SetConfigPath, got %d: %+v", len(configs), configs)
			}
			if len(configs) > 0 && configs[0].Foo != "initial" {
				t.Errorf("Expected first config.Foo = 'initial', got %q", configs[0].Foo)
			}
			return
		}

		// Fail fast if we get more than expected
		if len(configs) > 2 {
			t.Fatalf("Got too many broadcasts (%d), failing early: %+v", len(configs), configs)
		}
	}
}

// TestNoDualDeliveryOnFileChange tests that file changes don't cause duplicate broadcasts
func TestNoDualDeliveryOnFileChange(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	initial := []byte("foo: initial\n")
	if _, err := tmpfile.Write(initial); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	tmpfile.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	err = loader.SetConfigPath(tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("error setting config path: %v", err)
	}

	subscription := loader.Subscribe()

	// Consume the initial config
	initialConf := <-subscription
	if initialConf.Foo != "initial" {
		t.Errorf("Expected initial config.Foo = 'initial', got %q", initialConf.Foo)
	}

	// Give the watcher time to set up
	time.Sleep(100 * time.Millisecond)

	// Now change the file and ensure we get exactly one update
	updated := []byte("foo: updated\n")
	if err := os.WriteFile(tmpfile.Name(), updated, 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Collect broadcasts for the file change
	var configs []TestConf
	timeout := time.After(3 * time.Second) // Even longer timeout to ensure fsnotify has time to trigger

	for {
		select {
		case conf := <-subscription:
			configs = append(configs, conf)
			log.Printf("Received updated config #%d: %+v", len(configs), conf)

		case <-timeout:
			// Should have exactly one update
			if len(configs) != 1 {
				t.Errorf("Expected exactly 1 config broadcast from file change, got %d: %+v", len(configs), configs)
			}
			if len(configs) > 0 && configs[0].Foo != "updated" {
				t.Errorf("Expected updated config.Foo = 'updated', got %q", configs[0].Foo)
			}
			return
		}

		// Fail fast if we get more than expected
		if len(configs) > 2 {
			t.Fatalf("Got too many broadcasts from file change (%d), failing early: %+v", len(configs), configs)
		}
	}
}

// TestMultipleSubscribersGetSameConfig tests that all subscribers receive the same config updates
func TestMultipleSubscribersGetSameConfig(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	initial := []byte("foo: test\n")
	if _, err := tmpfile.Write(initial); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	tmpfile.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	err = loader.SetConfigPath(tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("error setting config path: %v", err)
	}

	// Create multiple subscribers
	const numSubs = 3
	var subscriptions []chan TestConf
	for i := 0; i < numSubs; i++ {
		subscriptions = append(subscriptions, loader.Subscribe())
	}

	// All should receive the initial config
	var initialConfigs []TestConf
	for i, sub := range subscriptions {
		select {
		case conf := <-sub:
			initialConfigs = append(initialConfigs, conf)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Subscriber %d didn't receive initial config", i)
		}
	}

	// Verify all got the same initial config
	for i, conf := range initialConfigs {
		if conf.Foo != "test" {
			t.Errorf("Subscriber %d got wrong initial config: %q", i, conf.Foo)
		}
	}

	// Give the watcher time to set up
	time.Sleep(100 * time.Millisecond)

	// Update the config
	updated := []byte("foo: updated\n")
	if err := os.WriteFile(tmpfile.Name(), updated, 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// All should receive the updated config
	var updatedConfigs []TestConf
	for i, sub := range subscriptions {
		select {
		case conf := <-sub:
			updatedConfigs = append(updatedConfigs, conf)
		case <-time.After(4 * time.Second):
			t.Fatalf("Subscriber %d didn't receive updated config", i)
		}
	}

	// Verify all got the same updated config
	for i, conf := range updatedConfigs {
		if conf.Foo != "updated" {
			t.Errorf("Subscriber %d got wrong updated config: %q", i, conf.Foo)
		}
	}
}

// TestNoDeliveryWhenConfigUnchanged tests that identical configs don't trigger broadcasts
func TestNoDeliveryWhenConfigUnchanged(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte("foo: unchanged\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	tmpfile.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	err = loader.SetConfigPath(tmpfile.Name(), true)
	if err != nil {
		t.Fatalf("error setting config path: %v", err)
	}

	subscription := loader.Subscribe()

	// Consume the initial config
	initialConf := <-subscription
	if initialConf.Foo != "unchanged" {
		t.Errorf("Expected initial config.Foo = 'unchanged', got %q", initialConf.Foo)
	}

	// Write the same content again (should not trigger broadcast)
	if err := os.WriteFile(tmpfile.Name(), content, 0644); err != nil {
		t.Fatalf("failed to rewrite config: %v", err)
	}

	// Should not receive any additional broadcasts
	select {
	case conf := <-subscription:
		t.Errorf("Unexpected config broadcast for unchanged file: %+v", conf)
	case <-time.After(1 * time.Second):
		// This is expected - no broadcast should occur
		log.Printf("Correctly received no broadcast for unchanged config")
	}
}

// TestConfigDeliveryAfterPathChange tests config delivery when changing paths
func TestConfigDeliveryAfterPathChange(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	// Create two temp files
	tmpfile1, err := os.CreateTemp("", "config1-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file 1: %v", err)
	}
	defer os.Remove(tmpfile1.Name())

	tmpfile2, err := os.CreateTemp("", "config2-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file 2: %v", err)
	}
	defer os.Remove(tmpfile2.Name())

	// Write different content to each
	if _, err := tmpfile1.Write([]byte("foo: file1\n")); err != nil {
		t.Fatalf("failed to write to file 1: %v", err)
	}
	tmpfile1.Close()

	if _, err := tmpfile2.Write([]byte("foo: file2\n")); err != nil {
		t.Fatalf("failed to write to file 2: %v", err)
	}
	tmpfile2.Close()

	loader, err := NewConfigLoader[TestConf]()
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	defer loader.Close()

	subscription := loader.Subscribe()

	// Set first config path
	err = loader.SetConfigPath(tmpfile1.Name(), true)
	if err != nil {
		t.Fatalf("error setting config path 1: %v", err)
	}

	// Should receive config from file1
	conf1 := <-subscription
	if conf1.Foo != "file1" {
		t.Errorf("Expected config from file1, got %q", conf1.Foo)
	}

	// Change to second config path
	err = loader.SetConfigPath(tmpfile2.Name(), true)
	if err != nil {
		t.Fatalf("error setting config path 2: %v", err)
	}

	// Should receive config from file2
	select {
	case conf2 := <-subscription:
		if conf2.Foo != "file2" {
			t.Errorf("Expected config from file2, got %q", conf2.Foo)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Did not receive config from file2")
	}

	// Verify no additional broadcasts
	select {
	case conf := <-subscription:
		t.Errorf("Unexpected additional broadcast: %+v", conf)
	case <-time.After(200 * time.Millisecond):
		// Expected - should be no more broadcasts
	}
}
