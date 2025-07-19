package configloader

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

type ConfigLoader[Config any] struct {
	mu       sync.Mutex
	path     string
	fprint   string
	conf     *Config
	control  chan string
	subs     []chan Config
	callback func(Config) (Config, error) // callback for config validation/transformation
}

func New[Config any](path string) (ret *ConfigLoader[Config], err error) {
	return NewConfigLoader[Config](path)
}

// This might return an error and a valid config loader.
func NewConfigLoader[Config any](path string) (ret *ConfigLoader[Config], err error) {
	ret = &ConfigLoader[Config]{
		control: make(chan string, 1),
	}
	if path != "" {
		ret.path = path
	}

	return ret, nil
}

// Close stops the config loader and closes the control channel.
func (b *ConfigLoader[Config]) Close() {
	b.control <- "done"
	close(b.control)
}

// Start begins the config loader's watch loop, which will reload the config periodically or on file changes.
func (b *ConfigLoader[Config]) Start() {
	// Periodically reload the config.
	go b.watch()
}

// Subscribe returns a channel that will receive updates when the config changes.
func (b *ConfigLoader[Config]) Subscribe() chan Config {
	ret := make(chan Config, 1)
	b.mu.Lock()
	if b.conf == nil && b.path != "" {
		b.mu.Unlock()
		_ = b.Load()
		b.mu.Lock()
	}
	b.subs = append(b.subs, ret)
	if b.conf != nil {
		ret <- *b.conf
	}
	b.mu.Unlock()
	return ret
}

// SetConfigPath updates the config path and reloads the config.
func (b *ConfigLoader[Config]) SetConfigPath(path string) error {
	b.mu.Lock()
	if b.path == path {
		b.mu.Unlock()
		return nil
	}
	b.path = path
	b.mu.Unlock()
	b.control <- "update"
	return b.Load()
}

// RegisterCallback sets a callback to be invoked with each new config. If the callback returns an error, the config is not used.
func (b *ConfigLoader[Config]) RegisterCallback(cb func(Config) (Config, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.callback = cb
}

// Load reads the config file, unmarshals it, and broadcasts it to subscribers.
func (b *ConfigLoader[Config]) Load() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.path == "" {
		return fmt.Errorf("no config path specified")
	}
	configBytes, err := os.ReadFile(b.path)
	if err != nil {
		return fmt.Errorf("could not read config @ %q: %v", b.path, err)
	}
	if len(configBytes) < 10 {
		return fmt.Errorf("empty or truncated config")
	}

	fprint := fmt.Sprintf("%x", sha256.Sum256(configBytes))
	if fprint == b.fprint {
		// Same as before, end early.
		return nil
	}

	conf := new(Config)
	err = yaml.Unmarshal(configBytes, conf)
	if err != nil {
		return fmt.Errorf("could not read config %q: %v", b.path, err)
	}

	// If callback is set, call it and use the returned config if no error
	if b.callback != nil {
		newConf, err := b.callback(*conf)
		if err != nil {
			log.Printf("config callback error: %v", err)
			return err
		}
		conf = &newConf
	}

	log.Printf("read config %q, with hash: %s", b.path, fprint)

	// store the config
	b.conf = conf
	b.fprint = fprint

	// broadcast
	for _, s := range b.subs {
		select {
		case s <- *conf:
		default:
			log.Println("subscriber channel is full")
		}
	}

	return nil
}

// Config returns the current config. If the config has not been loaded yet, it will attempt to load it
func (b *ConfigLoader[Config]) Config() (conf *Config) {
	b.mu.Lock()
	if b.conf == nil && b.path != "" {
		b.mu.Unlock()
		_ = b.Load()
		b.mu.Lock()
	}
	conf = b.conf
	b.mu.Unlock()
	return
}

func (b *ConfigLoader[Config]) watch() {

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify error: %v", err)
		log.Printf("polling config file: %s", b.path)
		for {
			select {
			case <-time.After(time.Second * 10):
				b.Load()
			case cmd := <-b.control:
				if cmd == "done" {
					log.Printf("exiting config pool loop")
					return
				}
			}
		}
	}

	defer w.Close()

	b.mu.Lock()
	path := b.path
	b.mu.Unlock()

	log.Printf("watching config file: %s", b.path)
	w.Add(filepath.Dir(path))
	for {
		select {
		case cmd := <-b.control:
			if cmd == "done" {
				log.Printf("exiting config pool loop")
				return
			}
			if cmd == "update" {
				oldpath := path
				b.mu.Lock()
				path = b.path
				b.mu.Unlock()
				log.Printf("updating config watch path to: %q", path)
				w.Remove(filepath.Dir(oldpath))
				w.Add(filepath.Dir(b.path))
			}
		case _, ok := <-w.Errors:
			if !ok {
				log.Printf("fsnotify closed")
				return
			}
			log.Printf("fsnotify error: %v", err)
		case event, ok := <-w.Events:
			if !ok {
				log.Printf("fsnotify closed")
				return
			}
			if event.Has(fsnotify.Write) {
				b.Load()
			}
		case <-time.After(time.Second * 10):
			b.Load()
		}
	}
}
