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
	required bool // if true, will return an error if no config is found
	fprint   string
	conf     *Config
	control  chan string
	subs     []chan Config
	callback func(Config) (Config, error) // callback for config validation/transformation
}

// New creates a new ConfigLoader instance.
// If you want to set a path, use SetConfigPath after creation.
// If no path is set it will return the default config (zero value of Config),
// as modified by a registered callback, which can set default values.
func New[Config any]() (ret *ConfigLoader[Config], err error) {
	return NewConfigLoader[Config]()
}

// NewConfigLoader creates a new ConfigLoader instance.
func NewConfigLoader[Config any]() (ret *ConfigLoader[Config], err error) {
	ret = &ConfigLoader[Config]{
		control: make(chan string, 1),
	}
	// Periodically reload the config.
	go ret.watch()
	return ret, nil
}

// Close stops the config loader and closes the control channel.
func (b *ConfigLoader[Config]) Close() {
	b.control <- "done"
	close(b.control)
}

// Subscribe returns a channel that will receive updates when the config changes.
func (b *ConfigLoader[Config]) Subscribe() chan Config {
	ret := make(chan Config, 1)
	b.mu.Lock()
	b.subs = append(b.subs, ret)
	conf := b.conf
	b.mu.Unlock()
	if conf != nil {
		ret <- *conf
	}
	return ret
}

// SetConfigPath updates the config path and, if the path changed, reloads the config.
// Returns an error if the config file annot be loaded.
func (b *ConfigLoader[Config]) SetConfigPath(path string, required bool) error {
	b.mu.Lock()
	// No-op
	if b.path == path && b.required == required {
		b.mu.Unlock()
		return nil
	}
	b.required = required
	b.path = path
	b.mu.Unlock()
	b.control <- "update"
	err := b.Load()
	if err != nil {
		log.Printf("config path set to: %s (required: %v), error loading:%v", path, required, err)
	} else {
		log.Printf("config path set to: %s (required: %v), loaded", path, required)
	}
	return err
}

// RegisterCallback sets a callback to be invoked with each new config. If the callback returns an error, the config is not used.
func (b *ConfigLoader[Config]) RegisterCallback(cb func(Config) (Config, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.callback = cb
}

func (b *ConfigLoader[Config]) defaultConfig() (Config, error) {
	var zero Config
	if b.callback == nil {
		return zero, nil
	}
	return b.callback(zero)
}

func (b *ConfigLoader[Config]) DefaultConfig() (Config, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.defaultConfig()
}

// Load reads the config file, unmarshals it, and broadcasts it to subscribers.
func (b *ConfigLoader[Config]) Load() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// if there is no path set, use the zero value of Config.
	if b.path == "" && !b.required {
		log.Printf("no config path set, using zero value")
		zero, err := b.defaultConfig()
		if err != nil {
			log.Printf("error getting default config: %v", err)
			return err
		}
		b.conf = &zero
		// Serialize the zero config to YAML and fingerprint it
		yamlBytes, err := yaml.Marshal(zero)
		if err != nil {
			log.Printf("could not marshal zero config: %v", err)
			b.fprint = ""
		} else {
			b.fprint = fmt.Sprintf("%x", sha256.Sum256(yamlBytes))
		}
		log.Printf("default config with hash: %s", b.fprint)
		// broadcast
		for _, s := range b.subs {
			select {
			case s <- zero:
			default:
				log.Println("subscriber channel is full")
			}
		}
		return nil
	}

	// If there is no path, but the config is required, return an error.
	// Weird case, but we want to be explicit about it.
	if b.path == "" && b.required {
		return fmt.Errorf("no config path set, but config is required")
	}

	// We have a path, so we can read the config file.
	configBytes, err := os.ReadFile(b.path)
	// successful file read; process the config.
	if err == nil {
		fprint := fmt.Sprintf("%x", sha256.Sum256(configBytes))
		if fprint == b.fprint {
			// Same as before, end early.
			return nil
		}

		// Deserialize the config
		conf := new(Config)
		err = yaml.Unmarshal(configBytes, conf)
		if err != nil {
			return fmt.Errorf("could not deserialize config %q: %v", b.path, err)
		}

		// If callback is set, call it and use the returned config if no error
		if b.callback != nil {
			newConf, err := b.callback(*conf)
			if err != nil {
				log.Printf("config callback error, rejecting config: %v", err)
				return err
			}
			conf = &newConf
		}

		log.Printf("read new config %q, with hash: %s", b.path, fprint)

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

	// Unsuccessful file read; if required, return an error.
	if b.conf == nil && b.required {
		return fmt.Errorf("could not read required config @ %q, no config available: %v", b.path, err)
	}

	// if we have previously loaded a config, we can use it.
	if b.conf != nil {
		log.Printf("still using previous config, with hash: %s", b.fprint)
		return nil
	}

	// If not required, use the default config, even if the file is busted.
	zero, err := b.defaultConfig()
	if err != nil {
		log.Printf("error getting default config: %v", err)
		return err
	}
	yamlBytes, err := yaml.Marshal(zero)
	if err != nil {
		log.Printf("error serializing default config: %v", err)
		return err
	}
	fprint := fmt.Sprintf("%x", sha256.Sum256(yamlBytes))
	b.conf = &zero
	b.fprint = fprint
	log.Printf("using default config with hash: %s", b.fprint)

	return nil
}

// Config returns the current config. If the config has not been loaded yet, it will attempt to load it
func (b *ConfigLoader[Config]) Config() (conf *Config) {
	b.mu.Lock()
	if b.conf == nil {
		b.mu.Unlock()
		err := b.Load()
		if err != nil {
			log.Printf("error loading config: %v", err)
		}
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
		log.Printf("falling back to polling config file: %s", b.path)
		for {
			select {
			case <-time.After(time.Second * 2):
				// Only poll if we have a path to watch
				b.mu.Lock()
				hasPath := b.path != ""
				b.mu.Unlock()
				if hasPath {
					b.Load()
				}
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

	// Only start watching if we have a path
	if path != "" {
		log.Printf("watching config file: %s", path)
		w.Add(filepath.Dir(path))
	}

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
				if oldpath != "" {
					w.Remove(filepath.Dir(oldpath))
				}
				if path != "" {
					w.Add(filepath.Dir(path))
				}
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
				log.Printf("config file changed: %s", event.Name)
				b.Load()
			}
		case <-time.After(time.Second * 2):
			// Only poll if we have a path to watch
			b.mu.Lock()
			hasPath := b.path != ""
			b.mu.Unlock()
			if hasPath {
				b.Load()
			}
		}
	}
}
