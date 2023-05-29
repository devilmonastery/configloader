package jsonexporter

import (
	"testing"
)

type TestConf struct {
	Foo string
	Bar string
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
	if conf.Bar != "bar!" {
		t.Errorf("expected 'bar' = 'bar!', got %q", conf.Bar)
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
	if conf.Bar != "bar!" {
		t.Errorf("expected 'bar' = 'bar!', got %q", conf.Bar)
	}
}
