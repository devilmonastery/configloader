package configloader

import (
	"log"
	"os"
	"testing"
	"time"
)

type TestConf struct {
	Foo string
}

func TestLoadConfig(t *testing.T) {
	loader, err := NewConfigLoader[TestConf]("testdata/config.yaml")
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}

	conf := loader.Config()

	if conf.Foo != "foo!" {
		t.Errorf("expected 'foo' = 'foo!', got %q", conf.Foo)
	}
}

func TestSubscribeConfig(t *testing.T) {
	loader, err := NewConfigLoader[TestConf]("testdata/config.yaml")
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	if loader == nil {
		t.Fatalf("error creating config loader")
	}

	ch := loader.Subscribe()

	conf := <-ch

	if conf.Foo != "foo!" {
		t.Errorf("expected 'foo' = 'foo!', got %q", conf.Foo)
	}
}

func TestSubscribeConfigWithTempFile(t *testing.T) {
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

	loader, err := NewConfigLoader[TestConf](tmpfile.Name())
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
