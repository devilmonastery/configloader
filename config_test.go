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

	log.Printf("1")
	err = loader.SetConfigPath("testdata/noconfig.yaml", true)
	if err == nil {
		t.Fatalf("expected error loading missing config")
	}

	log.Printf("2")
	conf := loader.Config()
	if conf != nil {
		t.Fatalf("expected nil config, got %v", conf)
	}

	log.Printf("3")
	err = loader.SetConfigPath("testdata/noconfig.yaml", false)
	log.Printf("err: %v", err)
	if err == nil {
		t.Fatalf("expected error loading missing config")
	}

	log.Printf("4")
	conf = loader.Config()
	if conf != nil {
		t.Fatalf("expected nil config, got %v", conf)
	}

	log.Printf("5")
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
