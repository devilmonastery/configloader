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

// This might return an error and a valid config loader.
func NewConfigLoader[Config any](path string) (ret *ConfigLoader[Config], err error) {
	//log.Printf("NewBotConfigLoader")
	ret = &ConfigLoader[Config]{
		control: make(chan string, 1),
	}

	err = ret.Load(path)
	if err != nil {
		log.Printf("config error: %v", err)
	}

	// Periodically reload the config.
	go ret.watch()

	return
}

func (b *ConfigLoader[Config]) Close() {
	b.control <- "done"
	close(b.control)
}

func (b *ConfigLoader[Config]) Subscribe() chan Config {
	ret := make(chan Config, 1)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, ret)
	ret <- *b.conf
	return ret
}

func (b *ConfigLoader[Config]) SetConfigPath(path string) error {
	b.mu.Lock()
	if b.path == path {
		return nil
	}
	b.mu.Unlock()
	b.control <- "update"
	return b.Load(path)
}

// RegisterCallback sets a callback to be invoked with each new config. If the callback returns an error, the config is not used.
func (b *ConfigLoader[Config]) RegisterCallback(cb func(Config) (Config, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.callback = cb
}

func (b *ConfigLoader[Config]) Load(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if path != "" {
		b.path = path
	}

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

func (b *ConfigLoader[Config]) watch() {

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify error: %v", err)
		log.Printf("polling config file: %s", b.path)
		for {
			select {
			case <-time.After(time.Second * 10):
				b.Load("")
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
				b.Load("")
			}
		case <-time.After(time.Second * 10):
			b.Load("")
		}
	}
}

func (b *ConfigLoader[Config]) Config() (conf *Config) {
	b.mu.Lock()
	defer b.mu.Unlock()
	conf = b.conf
	return
}
